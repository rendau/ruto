package core

import (
	"time"

	configCore "github.com/rendau/ruto/internal/config/core"
	"github.com/rendau/ruto/internal/constant"
	changeRequestModel "github.com/rendau/ruto/internal/domain/changerequest/model"
	handlerSystemP "github.com/rendau/ruto/internal/handler/system"
	"github.com/rendau/ruto/internal/infra/pulsekit"
)

// pulseServiceName is core's name in the pulse catalog (its image name), not
// constant.ServiceName: another name would read as a rename there.
const pulseServiceName = "ruto-core"

// newPulsekit — the manifest for pulse (github.com/rendau/pulse, docs/service-manifest.md).
// pulse talks to core only: what it knows about the gateways comes from their
// heartbeats to core. Dependencies are declared in Init, the rest — in handler/system.
func newPulsekit() *pulsekit.Kit {
	kit := pulsekit.New(pulsekit.Config{}, pulsekit.Service{
		Name:  pulseServiceName,
		Title: "ruto — управление API-шлюзом",
		Description: "Хранит конфигурацию API-шлюза ruto: приложения (префикс пути → сервис), маршруты, авторизацию, " +
			"переменные. Даёт админку и MCP для правки, публикует конфигурацию на шлюзы ruto-gateway и знает, " +
			"какую версию получил каждый шлюз.",
		Aliases:     []string{"ruto", "руто", "админка ruto", "api-gateway", "апи шлюз"},
		OwnerTeam:   "Даурен М",
		Criticality: "high",
		RepoUrl:     "https://github.com/rendau/ruto",
		Domain: &pulsekit.Domain{
			Responsibilities: []string{
				"Хранит конфигурацию API-шлюза: приложения (префикс пути → сервис), маршруты, авторизацию, переменные",
				"Публикует конфигурацию на шлюзы ruto-gateway и знает, какую версию получил каждый шлюз",
				"Принимает правки из админки и запросы на изменение от ИИ-агентов, которые одобряет человек",
			},
			NotResponsible: []pulsekit.Boundary{
				{What: "приём и проксирование внешнего трафика", Service: "ruto-gateway"},
			},
			Entities: []pulsekit.Entity{{
				Name:        handlerSystemP.ChangeRequestEntity,
				IdPattern:   "[0-9]{1,12}",
				IdExample:   "42",
				Description: "правка конфигурации от ИИ-агента: ждёт решения человека в админке ruto",
				Statuses: []pulsekit.EntityStatus{
					{Name: changeRequestModel.StatusPending, Meaning: "ждёт решения человека", StuckAfter: 24 * time.Hour},
					{Name: changeRequestModel.StatusApplied, Meaning: "применён"},
					{Name: changeRequestModel.StatusRejected, Meaning: "отклонён человеком"},
					{Name: changeRequestModel.StatusOutdated, Meaning: "объект изменился после предложения, применить нельзя"},
					{Name: changeRequestModel.StatusExpired, Meaning: "никто не решил за отведённый срок"},
					{Name: changeRequestModel.StatusCancelled, Meaning: "отозван агентом"},
				},
			}},
			Questions: []pulsekit.Question{
				{Question: "дошли ли изменения до шлюзов, все ли шлюзы на связи", Endpoint: "gateways"},
				{Question: "почему новый маршрут не работает", Endpoint: "gateways"},
			},
		},
	}, pulsekit.Build{})

	// traffic through the gateways: their metrics, read from prometheus — pulse
	// does not go to the gateways themselves
	prefix := configCore.Conf.MetricsNamespace + "_" + constant.ServiceName + "_gw_http_"
	kit.Metric(pulsekit.Metric{
		Id: "gateway_rps", Title: "Запросов в секунду через шлюзы", Unit: "rps", Direction: "higher_is_better",
		PromQL: "sum(rate(" + prefix + "requests_total[5m]))",
	})
	kit.Metric(pulsekit.Metric{
		Id: "gateway_5xx", Title: "Доля ответов 5xx через шлюзы", Unit: "ratio", Direction: "lower_is_better",
		PromQL: "sum(rate(" + prefix + `requests_total{status=~"5.."}[5m])) / sum(rate(` + prefix + "requests_total[5m]))",
	})
	kit.Metric(pulsekit.Metric{
		Id: "gateway_p95", Title: "Задержка p95 через шлюзы", Unit: "seconds", Direction: "lower_is_better",
		PromQL: "histogram_quantile(0.95, sum by (le) (rate(" + prefix + "request_duration_seconds_bucket[5m])))",
	})

	return kit
}
