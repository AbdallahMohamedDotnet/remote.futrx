package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"

	"futrx.local/catalog/applications/file-manager/backend/container/internal/config"
	presentation "futrx.local/catalog/applications/file-manager/backend/container/internal/presentation/http"
	"futrx.local/catalog/applications/file-manager/backend/container/internal/presentation/web"
	"futrx.local/catalog/applications/file-manager/backend/container/internal/repository/archives"
	"futrx.local/catalog/applications/file-manager/backend/container/internal/repository/workspace"
	"futrx.local/catalog/applications/file-manager/backend/container/internal/service/files"
)

func main() {
	port := flag.Int("port", config.DefaultPort, "HTTP port")
	flag.Parse()
	if *port < 1 || *port > 65535 {
		log.Fatal("invalid port")
	}
	spool := archives.New(config.MaxConcurrentArchives, config.MaxArchiveSpoolBytes)
	service := files.New(config.WorkspaceDirectory, workspace.New(), spool)
	server := &http.Server{
		Addr:              "0.0.0.0:" + strconv.Itoa(*port),
		Handler:           presentation.New(service, web.Files()),
		ReadHeaderTimeout: config.ReadHeaderTimeout,
		IdleTimeout:       config.IdleTimeout,
	}
	log.Fatal(server.ListenAndServe())
}
