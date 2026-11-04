package allapis

import (
	"fmt"
	"sort"
	"strings"
)

// Domains agrupa las APIs por contexto de negocio. Cada dominio corre en su
// propio proceso (un Deployment) con API_DOMAIN=<dominio>. La primera API de
// cada lista responde /health.
var Domains = map[string][]string{
	// TAL-107. service va aquí aunque la tarea no la nombre: review la llama
	// y es parte del catálogo (auditoría TAL-105).
	"comercial": {"product", "store", "service", "pricing", "search", "comparison",
		"favorites", "cart", "review", "watchlist", "notification"},
	// TAL-106. Cuentas y sesiones; nadie las llama por HTTP: las demás APIs
	// validan la sesión leyendo user_session.
	"usuarios": {"auth", "user"},
	// El scraper escribe aquí; avisa bajas de precio a notification (comercial).
	"ingestion": {"ingestion"},
}

// Calls son las llamadas HTTP entre APIs: quién llama a quién (auditoría
// TAL-105 §5). Si una API empieza a llamar a otra, hay que agregarla aquí.
var Calls = map[string][]string{
	"favorites": {"product"},
	"cart":      {"product", "pricing"},
	"review":    {"product", "store", "service"},
	"watchlist": {"pricing"},
	"pricing":   {"notification"},
	"ingestion": {"notification"},
}

// URLEnv es la variable con la URL base de cada API que otra llama.
var URLEnv = map[string]string{
	"product":      "PRODUCT_API_URL",
	"store":        "STORE_API_URL",
	"service":      "SERVICE_API_URL",
	"pricing":      "PRICING_API_URL",
	"notification": "NOTIFICATION_API_URL",
}

// APIs devuelve las APIs que sirve el proceso: las del dominio, o las 14 si
// domain es "" (modo consolidado).
func APIs(domain string) ([]string, error) {
	if domain == "" {
		return allAPIs, nil
	}
	apis, ok := Domains[domain]
	if !ok {
		names := make([]string, 0, len(Domains))
		for name := range Domains {
			names = append(names, name)
		}
		sort.Strings(names)
		return nil, fmt.Errorf("API_DOMAIN=%q no existe; usa uno de: %s, o déjala vacía para las 14 APIs", domain, strings.Join(names, ", "))
	}
	return apis, nil
}

// InternalURLs resuelve las URLs de las llamadas que hacen las APIs del
// proceso. Si la API llamada está en el mismo proceso, su URL es self (salvo
// que ya esté definida). Si está en otro proceso, requiere su URL específica
// o GATEWAY_URL: el default localhost de cada cliente apunta al propio
// contenedor y la llamada fallaría.
func InternalURLs(apis []string, self string, getenv func(string) string) (map[string]string, error) {
	inside := make(map[string]bool, len(apis))
	for _, api := range apis {
		inside[api] = true
	}
	urls := map[string]string{}
	var missing []string
	for _, api := range apis {
		for _, callee := range Calls[api] {
			name := URLEnv[callee]
			switch {
			case getenv(name) != "":
			case inside[callee]:
				urls[name] = self
			case strings.TrimSpace(getenv("GATEWAY_URL")) != "":
				urls[name] = strings.TrimRight(strings.TrimSpace(getenv("GATEWAY_URL")), "/")
			default:
				missing = append(missing, fmt.Sprintf("%s (%s llama a %s)", name, api, callee))
			}
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("faltan URLs de APIs de otro dominio: %s", strings.Join(missing, ", "))
	}
	return urls, nil
}
