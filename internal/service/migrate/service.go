package migrate

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/constant"
	appModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
)

type Service struct {
	client      *legacyClient
	rootSvc     RootServiceI
	appSvc      AppServiceI
	endpointSvc EndpointServiceI
}

type Result struct {
	RealmName     string
	RootBaseURL   string
	AppCount      int64
	EndpointCount int64
}

type migratedData struct {
	root      *rootModel.Root
	apps      []*appModel.App
	endpoints []*endpointModel.Endpoint
}

type RootServiceI interface {
	Set(ctx context.Context, obj *rootModel.Root) error
}

type AppServiceI interface {
	List(ctx context.Context, pars *appModel.ListReq) ([]*appModel.App, int64, error)
	Create(ctx context.Context, obj *appModel.App) (string, error)
	Delete(ctx context.Context, id string) error
}

type EndpointServiceI interface {
	Create(ctx context.Context, obj *endpointModel.Endpoint) (string, error)
}

func New(
	baseURL, refreshToken string,
	rootSvc RootServiceI,
	appSvc AppServiceI,
	endpointSvc EndpointServiceI,
) *Service {
	return &Service{
		client:      newLegacyClient(baseURL, refreshToken),
		rootSvc:     rootSvc,
		appSvc:      appSvc,
		endpointSvc: endpointSvc,
	}
}

func (s *Service) Run(ctx context.Context, realmName, jwkURL string) (*Result, error) {
	if strings.TrimSpace(s.client.baseURL) == "" {
		return nil, fmt.Errorf("legacy base url: empty")
	}
	if strings.TrimSpace(s.client.refreshToken) == "" {
		return nil, fmt.Errorf("legacy refresh token: empty")
	}
	if s.rootSvc == nil {
		return nil, fmt.Errorf("rootSvc: nil")
	}
	if s.appSvc == nil {
		return nil, fmt.Errorf("appSvc: nil")
	}
	if s.endpointSvc == nil {
		return nil, fmt.Errorf("endpointSvc: nil")
	}

	data, err := s.fetchAndMap(ctx, realmName, jwkURL)
	if err != nil {
		return nil, fmt.Errorf("fetchAndMap: %w", err)
	}

	if err = s.persist(ctx, data); err != nil {
		return nil, fmt.Errorf("persist: %w", err)
	}

	return &Result{
		RealmName:     strings.TrimSpace(realmName),
		RootBaseURL:   data.root.BaseUrl,
		AppCount:      int64(len(data.apps)),
		EndpointCount: int64(len(data.endpoints)),
	}, nil
}

func (s *Service) fetchAndMap(ctx context.Context, realmName, jwkURL string) (*migratedData, error) {
	accessToken, err := s.client.authByRefreshToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("authByRefreshToken: %w", err)
	}

	realms, err := s.client.listRealms(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("listRealms: %w", err)
	}

	targetRealm, err := pickRealmByName(realms, realmName)
	if err != nil {
		return nil, fmt.Errorf("pickRealmByName: %w", err)
	}

	apps, err := s.client.listApps(ctx, accessToken, targetRealm.Id)
	if err != nil {
		return nil, fmt.Errorf("listApps: %w", err)
	}

	apps = lo.FilterMap(apps, func(app legacyApp, _ int) (legacyApp, bool) {
		return app, strings.TrimSpace(app.Data.Path) != ""
	})

	endpoints := make([]legacyEndpoint, 0)
	for _, app := range apps {
		appEndpoints, getErr := s.client.listEndpoints(ctx, accessToken, app.Id)
		if getErr != nil {
			return nil, fmt.Errorf("listEndpoints app_id=%s: %w", app.Id, getErr)
		}
		endpoints = append(endpoints, appEndpoints...)
	}

	rootObj := mapRoot(targetRealm)
	if strings.TrimSpace(jwkURL) != "" {
		rootObj.Jwt = []rootModel.RootJwt{
			{JwkUrl: strings.TrimSpace(jwkURL)},
		}
	}

	appsObj := make([]*appModel.App, 0, len(apps))
	for _, item := range apps {
		appObj, mapErr := mapApp(item)
		if mapErr != nil {
			return nil, fmt.Errorf("mapApp id=%s: %w", item.Id, mapErr)
		}
		appsObj = append(appsObj, appObj)
	}

	rootJWTKids, err := s.resolveRootJWTKids(ctx, rootObj)
	if err != nil {
		return nil, fmt.Errorf("resolveRootJWTKids: %w", err)
	}
	rootObj.Auth = buildRootAuth(rootJWTKids)
	if len(rootJWTKids) == 0 {
		return nil, fmt.Errorf("root jwt kids: empty")
	}

	endpointsObj := make([]*endpointModel.Endpoint, 0, len(endpoints))
	for _, item := range endpoints {
		endpointObj, mapErr := mapEndpoint(item, rootJWTKids)
		if mapErr != nil {
			return nil, fmt.Errorf("mapEndpoint id=%s: %w", item.Id, mapErr)
		}
		endpointsObj = append(endpointsObj, endpointObj)
	}

	if err = rootObj.Normalize(); err != nil {
		return nil, fmt.Errorf("root.Normalize: %w", err)
	}
	for _, item := range appsObj {
		if err = item.Normalize(); err != nil {
			return nil, fmt.Errorf("app.Normalize id=%s: %w", item.Id, err)
		}
	}
	for _, item := range endpointsObj {
		if err = item.Normalize(); err != nil {
			return nil, fmt.Errorf("endpoint.Normalize id=%s: %w", item.Id, err)
		}
	}

	return &migratedData{
		root:      rootObj,
		apps:      appsObj,
		endpoints: endpointsObj,
	}, nil
}

func (s *Service) resolveRootJWTKids(ctx context.Context, rootObj *rootModel.Root) ([]string, error) {
	kidsMap := make(map[string]struct{})

	for _, jwtConf := range rootObj.Jwt {
		if strings.TrimSpace(jwtConf.JwkUrl) == "" {
			continue
		}
		kids, err := s.client.fetchJWKKids(ctx, jwtConf.JwkUrl)
		if err != nil {
			return nil, fmt.Errorf("fetchJWKKids %s: %w", jwtConf.JwkUrl, err)
		}
		for _, kid := range kids {
			kid = strings.TrimSpace(kid)
			if kid != "" {
				kidsMap[kid] = struct{}{}
			}
		}
	}

	result := lo.Keys(kidsMap)
	sort.Strings(result)

	return result, nil
}

func (s *Service) persist(ctx context.Context, data *migratedData) error {
	errs := make([]string, 0)
	addErr := func(format string, args ...any) {
		errs = append(errs, fmt.Sprintf(format, args...))
	}

	appsForDelete, _, err := s.appSvc.List(ctx, &appModel.ListReq{})
	if err != nil {
		addErr("appSvc.List: %v", err)
	}
	if err == nil {
		for _, item := range appsForDelete {
			if err = s.appSvc.Delete(ctx, item.Id); err != nil {
				addErr("appSvc.Delete id=%s: %v", item.Id, err)
			}
		}
	}

	if err = s.rootSvc.Set(ctx, rootModel.NewEmpty()); err != nil {
		addErr("rootSvc.Set empty: %v", err)
	}

	for _, item := range data.apps {
		createdID, createErr := s.appSvc.Create(ctx, item)
		if createErr != nil {
			addErr("appSvc.Create id=%s: %v", item.Id, createErr)
			continue
		}
		if strings.TrimSpace(createdID) != strings.TrimSpace(item.Id) {
			addErr("appSvc.Create id mismatch: expected=%s actual=%s", item.Id, createdID)
		}
	}

	for _, item := range data.endpoints {
		createdID, createErr := s.endpointSvc.Create(ctx, item)
		if createErr != nil {
			addErr("endpointSvc.Create id=%s: %v", item.Id, createErr)
			continue
		}
		if strings.TrimSpace(createdID) != strings.TrimSpace(item.Id) {
			addErr("endpointSvc.Create id mismatch: expected=%s actual=%s", item.Id, createdID)
		}
	}

	if err = s.rootSvc.Set(ctx, data.root); err != nil {
		addErr("rootSvc.Set final: %v", err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("persist finished with errors (%d): %s", len(errs), strings.Join(errs, " | "))
	}

	return nil
}

func pickRealmByName(realms []legacyRealm, realmName string) (*legacyRealm, error) {
	targetName := strings.TrimSpace(realmName)
	var found *legacyRealm

	for i := range realms {
		name := strings.TrimSpace(realms[i].Data.Name)
		if name != targetName {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("realm with name %q is not unique", targetName)
		}
		found = &realms[i]
	}
	if found == nil {
		return nil, fmt.Errorf("realm with name %q not found", targetName)
	}
	return found, nil
}

func mapRoot(item *legacyRealm) *rootModel.Root {
	result := rootModel.NewEmpty()

	result.BaseUrl = item.Data.PublicBaseURL
	result.Cors = rootModel.RootCors{
		Enabled:          item.Data.CorsConf.Enabled,
		AllowCredentials: item.Data.CorsConf.AllowCredentials,
		MaxAge:           item.Data.CorsConf.MaxAge,
		AllowOrigins:     item.Data.CorsConf.AllowOrigins,
		AllowMethods:     item.Data.CorsConf.AllowMethods,
		AllowHeaders:     item.Data.CorsConf.AllowHeaders,
	}
	if strings.TrimSpace(item.Data.JWTConf.JWKURL) != "" {
		result.Jwt = []rootModel.RootJwt{
			{JwkUrl: item.Data.JWTConf.JWKURL},
		}
	}
	result.Auth = authModel.Auth{Enabled: false, Mode: constant.AuthModeExtend, Methods: []*authModel.AuthMethod{}}

	return result
}

func mapApp(item legacyApp) (*appModel.App, error) {
	backendURL, err := composeBackendURL(item.Data.BackendBase.Host, item.Data.BackendBase.Path)
	if err != nil {
		return nil, fmt.Errorf("composeBackendURL: %w", err)
	}

	return &appModel.App{
		Id:         item.Id,
		Active:     item.Active,
		PathPrefix: item.Data.Path,
		Name:       item.Data.Name,
		Backend: appModel.AppBackend{
			Url: backendURL,
		},
		Auth: authModel.Auth{
			Enabled: true,
			Mode:    constant.AuthModeExtend,
			Methods: []*authModel.AuthMethod{},
		},
		Endpoints: []*endpointModel.Endpoint{},
	}, nil
}

func composeBackendURL(host, pathValue string) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", fmt.Errorf("backend_base.host: empty")
	}

	u, err := url.Parse(host)
	if err != nil {
		return "", fmt.Errorf("url.Parse host: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("backend_base.host: scheme must be http/https")
	}
	if strings.TrimSpace(u.Host) == "" {
		return "", fmt.Errorf("backend_base.host: host is empty")
	}

	extraPath := strings.Trim(strings.TrimSpace(pathValue), "/")
	basePath := strings.TrimSuffix(strings.TrimSpace(u.Path), "/")
	switch {
	case basePath == "" && extraPath == "":
		u.Path = ""
	case basePath == "":
		u.Path = "/" + extraPath
	case extraPath == "":
		u.Path = basePath
	default:
		u.Path = basePath + "/" + extraPath
	}
	return u.String(), nil
}

func mapEndpoint(item legacyEndpoint, rootJWTKids []string) (*endpointModel.Endpoint, error) {
	customPath := ""
	if item.Data.Backend.CustomPath {
		customPath = item.Data.Backend.Path
	}

	hasJWT := item.Data.JWTValidation.Enabled
	hasIP := item.Data.IPValidation.Enabled

	endpointAuth := authModel.Auth{
		Enabled: true,
		Mode:    constant.AuthModeExtend,
		Methods: []*authModel.AuthMethod{},
	}

	switch {
	case !hasJWT && !hasIP:
		// Public endpoint: explicit auth replace
		endpointAuth = authModel.Auth{
			Enabled: false,
			Mode:    constant.AuthModeReplace,
			Methods: []*authModel.AuthMethod{},
		}

	case hasJWT && !hasIP && len(item.Data.JWTValidation.Roles) == 0:
		// Protected by default root JWT rules only
		endpointAuth = authModel.Auth{
			Enabled: true,
			Mode:    constant.AuthModeExtend,
			Methods: []*authModel.AuthMethod{},
		}

	default:
		// Endpoint has custom restrictions: replace and define explicit rules
		authMethods := make([]*authModel.AuthMethod, 0, len(rootJWTKids)+1)

		if hasJWT {
			if len(rootJWTKids) == 0 {
				return nil, fmt.Errorf("jwt_validation enabled but root has no jwt kids")
			}
			authMethods = append(authMethods, lo.Map(rootJWTKids, func(kid string, _ int) *authModel.AuthMethod {
				return &authModel.AuthMethod{
					JWT: &authModel.AuthMethodJWT{
						Kid:   kid,
						Roles: append(make([]string, 0, len(item.Data.JWTValidation.Roles)), item.Data.JWTValidation.Roles...),
					},
				}
			})...)
		}

		if hasIP {
			authMethods = append(authMethods, &authModel.AuthMethod{
				IPValidation: &authModel.AuthMethodIPValidation{
					AllowedIps: append(make([]string, 0, len(item.Data.IPValidation.AllowedIPs)), item.Data.IPValidation.AllowedIPs...),
				},
			})
		}

		endpointAuth = authModel.Auth{
			Enabled: len(authMethods) > 0,
			Mode:    constant.AuthModeReplace,
			Methods: authMethods,
		}
	}

	return &endpointModel.Endpoint{
		Id:     item.Id,
		AppId:  item.AppId,
		Active: item.Active,
		Method: item.Data.Method,
		Path:   item.Data.Path,
		Backend: endpointModel.Backend{
			CustomPath: customPath,
		},
		Auth: endpointAuth,
	}, nil
}

func buildRootAuth(kids []string) authModel.Auth {
	methods := lo.Map(kids, func(kid string, _ int) *authModel.AuthMethod {
		return &authModel.AuthMethod{
			JWT: &authModel.AuthMethodJWT{
				Kid:   kid,
				Roles: []string{},
			},
		}
	})

	return authModel.Auth{
		Enabled: len(methods) > 0,
		Mode:    constant.AuthModeExtend,
		Methods: methods,
	}
}
