package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"stereonearclip/internal/server"
	"stereonearclip/internal/stereo"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "HTTP listen address")
	sceneID := flag.String("scene", stereo.SceneIDs[0], "scene for -json output")
	asJSON := flag.Bool("json", false, "print one scene report as JSON and exit")
	flag.Parse()

	if *asJSON {
		scene, err := stereo.FindScene(*sceneID)
		if err != nil {
			log.Fatal(err)
		}
		report, err := stereo.Analyze(scene)
		if err != nil {
			log.Fatal(err)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			log.Fatal(err)
		}
		return
	}

	handler, err := server.New()
	if err != nil {
		log.Fatal(err)
	}
	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	fmt.Printf("per-eye near clip demo: http://%s\n", *addr)
	log.Fatal(httpServer.ListenAndServe())
}
