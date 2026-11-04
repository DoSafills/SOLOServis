// all-apis sirve varias APIs en un solo proceso (ver internal/allapis).
//
// API_DOMAIN elige cuáles: un dominio (por ejemplo comercial) o vacía para las
// 14 (modo consolidado). Las llamadas entre APIs del mismo proceso
// (PRODUCT_API_URL, PRICING_API_URL, ...) apuntan a él salvo que se definan;
// para las que van a otro dominio se requiere su URL o GATEWAY_URL.
package main

import (
	"log"
	"os"

	"github.com/DoSafills/SOLOServis/backend/internal/allapis"
	"github.com/DoSafills/SOLOServis/backend/internal/runner"
)

func main() {
	domain := os.Getenv("API_DOMAIN")
	apis, err := allapis.APIs(domain)
	if err != nil {
		log.Fatal(err)
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	urls, err := allapis.InternalURLs(apis, "http://127.0.0.1:"+port, os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	for name, url := range urls {
		_ = os.Setenv(name, url)
	}
	name := "All APIs"
	if domain != "" {
		name = "Dominio " + domain
	}
	runner.Run(name, allapis.NewDomainRouter(apis))
}
