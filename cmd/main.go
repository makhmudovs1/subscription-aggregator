package main

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/swaggo/http-swagger"
	"log"
	"net/http"
	"os"
	_ "subscription-aggregator/docs"
	"subscription-aggregator/internal/subscription"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	dbURL := os.Getenv("DB_URL")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	dbpool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer dbpool.Close()
	r := mux.NewRouter()

	r.HandleFunc("/subscriptions", subscription.CreateSubscriptionHandler(dbpool)).Methods("POST")
	r.HandleFunc("/subscriptions", subscription.GetAllSubscriptionsHandler(dbpool)).Methods("GET")
	r.HandleFunc("/subscriptions/total", subscription.GetTotalSubscriptionsHandler(dbpool)).Methods("GET")
	r.HandleFunc("/subscriptions/{id}", subscription.GetSubscriptionByIDHandler(dbpool)).Methods("GET")
	r.HandleFunc("/subscriptions/{id}", subscription.UpdateSubscriptionHandler(dbpool)).Methods("PUT")
	r.HandleFunc("/subscriptions/{id}", subscription.DeleteSubscriptionHandler(dbpool)).Methods("DELETE")
	r.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

	srv := &http.Server{
		Handler: r,
		Addr:    ":" + port,
	}
	fmt.Println("Server started on :8080")
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Unable to start server: %v\n", err)
	}
}
