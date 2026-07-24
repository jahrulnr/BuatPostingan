package main

import (
	"log"
	"os"

	webchatusecase "buatpostingan/internal/application/usecase/webchat"
	"buatpostingan/internal/config"
	httpdelivery "buatpostingan/delivery/http"
	"buatpostingan/internal/infrastructure/stub"
)

func main() {
	cfg := config.Load()

	uc := webchatusecase.New(
		stub.ThreadStore{},
		stub.ThreadLock{},
		stub.InterruptFlag{},
		stub.SpeakFloor{},
		stub.TurnRateLimit{},
		stub.DocsIndex{},
		stub.TurnWorker{},
	)

	srv := httpdelivery.NewServer(cfg, uc)
	if err := httpdelivery.ListenAndServe(srv); err != nil {
		log.Printf("server stopped: %v", err)
		os.Exit(1)
	}
}
