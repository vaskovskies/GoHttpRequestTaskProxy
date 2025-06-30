package main

import (
	"GoHttpRequestTaskProxy/internal/taskserver"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "GoHttpRequestTaskProxy/docs"
)

// @title GoHttpRequestTaskProxy
// @version 1.0
// @description This is an API that sends requests to third party services and stores the responses in a tasks database
// @host localhost:8080
// @BasePath /
func main() {
	server, err := taskserver.New()
	if err != nil {
		return
	}

	go func() {
		if err := server.Srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	quit := make(chan os.Signal, 1)

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutdown Server ...")

	if err := server.Srv.Shutdown(ctx); err != nil {
		log.Println("Server Shutdown:", err)
	}

	<-ctx.Done()
	log.Println("timeout of 5 seconds.")
	log.Println("Server exiting")

	close(server.Tasks)

	server.Wg.Wait()
	log.Println("All workers have exited.")

}
