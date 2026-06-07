package jwt

import (
	"sync"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// indefiniteSeen хранит уже залогированные бессрочные токены процессом.
// Глобал пакета (а не поле Jwt), чтобы пережить пересборку снапшота и быть
// общим для всех app/endpoint. Ключ: kid + "|" + sub.
var indefiniteSeen sync.Map

// markIndefinite проверяет отсутствие claim exp. При первом появлении пары
// (kid, sub) возвращает userId и firstSeen=true; повторы → firstSeen=false.
// Токены с exp игнорируются (возвращает "", false).
func markIndefinite(kid string, claims jwtv5.MapClaims) (userId string, firstSeen bool) {
	if _, ok := claims["exp"]; ok {
		return "", false
	}

	userId, _ = claims["sub"].(string)
	if userId == "" {
		userId = "<unknown>"
	}

	_, loaded := indefiniteSeen.LoadOrStore(kid+"|"+userId, struct{}{})

	return userId, !loaded
}
