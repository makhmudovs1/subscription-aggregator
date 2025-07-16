package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"net/http"
	"strings"
	"time"
)

type CreateSubscriptionRequest struct {
	ServiceName string    `json:"service_name"`
	Price       int       `json:"price"`
	UserID      uuid.UUID `json:"user_id"`
	StartDate   string    `json:"start_date"`         // "07-2025"
	EndDate     *string   `json:"end_date,omitempty"` // "12-2025"
}

type UpdateSubscriptionRequest struct {
	ServiceName string  `json:"service_name"`
	Price       int     `json:"price"`
	StartDate   string  `json:"start_date"`
	EndDate     *string `json:"end_date,omitempty"`
}

// @Summary      Создать новую подписку
// @Description  Создаёт запись о подписке для пользователя
// @Tags         subscriptions
// @Accept       json
// @Produce      json
// @Param        subscription  body  subscription.CreateSubscriptionRequest  true  "Данные подписки"
// @Success      201  {object}  map[string]interface{}
// @Failure      400  {string}  string  "invalid request"
// @Failure      500  {string}  string  "failed to insert subscription"
// @Router       /subscriptions [post]
func CreateSubscriptionHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateSubscriptionRequest
		// Decoding request body to structure
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("failed to decode body: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		startDate, err := time.Parse("01-2006", req.StartDate)
		if err != nil {
			log.Printf("invalid start_date '%s': %v", req.StartDate, err)
			http.Error(w, "invalid start date", http.StatusBadRequest)
			return
		}
		var endDatePtr *time.Time
		if req.EndDate != nil {
			ed, err := time.Parse("01-2006", *req.EndDate)
			if err != nil {
				http.Error(w, "invalid end date", http.StatusBadRequest)
				return
			}
			endDatePtr = &ed
		}

		id := uuid.New() // generating new ID for subscription
		_, err = db.Exec(context.Background(),
			`INSERT INTO subscriptions (id, service_name, price, user_id, start_date, end_date)
             VALUES ($1, $2, $3, $4, $5, $6)`,
			id, req.ServiceName, req.Price, req.UserID, startDate, endDatePtr,
		)
		if err != nil {
			log.Printf("failed to insert subscription: %v", err)
			http.Error(w, "failed to insert subscription", http.StatusInternalServerError)
			return
		}
		log.Printf("created subscription: %s", id)

		// Sending the ans
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": id,
		})
	}
}

// @Summary      Получить все подписки
// @Description  Возвращает список всех подписок
// @Tags         subscriptions
// @Produce      json
// @Success      200  {array}   subscription.Subscription
// @Failure      500  {string}  string  "failed to get subscriptions"
// @Router       /subscriptions [get]
func GetAllSubscriptionsHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(context.Background(),
			`SELECT id, service_name, price, user_id, start_date, end_date FROM subscriptions`)
		if err != nil {
			http.Error(w, "failed to get subscriptions", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		subs := make([]Subscription, 0)
		for rows.Next() {
			var sub Subscription
			err := rows.Scan(&sub.ID, &sub.ServiceName, &sub.Price, &sub.UserID, &sub.StartDate, &sub.EndDate)
			if err != nil {
				http.Error(w, "scan error", http.StatusInternalServerError)
				return
			}
			subs = append(subs, sub)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(subs)
	}
}

// @Summary      Получить сумму всех подписок
// @Description  Считает сумму всех подписок c фильтрами по user_id, service_name и start_date
// @Tags         subscriptions
// @Produce      json
// @Param        user_id      query   string  false  "ID пользователя (UUID)"
// @Param        service_name query   string  false  "Название сервиса"
// @Param        start_date   query   string  false  "Дата начала (ММ-ГГГГ)"
// @Success      200  {object}  map[string]int  "total сумма"
// @Failure      400  {string}  string  "invalid start_date format"
// @Failure      500  {string}  string  "db error"
// @Router       /subscriptions/total [get]
func GetTotalSubscriptionsHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		userIDStr := q.Get("user_id")
		serviceName := q.Get("service_name")
		startDateStr := q.Get("start_date")

		conditions := []string{}
		args := []interface{}{}
		idx := 1

		if userIDStr != "" {
			conditions = append(conditions, fmt.Sprintf("user_id = $%d", idx))
			args = append(args, userIDStr)
			idx++
		}
		if serviceName != "" {
			conditions = append(conditions, fmt.Sprintf("service_name = $%d", idx))
			args = append(args, serviceName)
			idx++
		}
		if startDateStr != "" {
			t, err := time.Parse("01-2006", startDateStr)
			if err != nil {
				http.Error(w, "invalid start_date format", http.StatusBadRequest)
				return
			}
			conditions = append(conditions, fmt.Sprintf("start_date >= $%d", idx))
			args = append(args, t)
			idx++
		}
		sql := "SELECT COALESCE(SUM(price), 0) FROM subscriptions"
		if len(conditions) > 0 {
			sql += " WHERE " + strings.Join(conditions, " AND ")
		}
		var total int
		err := db.QueryRow(context.Background(), sql, args...).Scan(&total)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{
			"total": total,
		})
	}
}

// @Summary      Получить подписку по ID
// @Description  Возвращает данные подписки по её id
// @Tags         subscriptions
// @Produce      json
// @Param        id   path   string  true  "ID подписки (UUID)"
// @Success      200  {object}  subscription.Subscription
// @Failure      404  {string}  string  "subscription not found"
// @Router       /subscriptions/{id} [get]
func GetSubscriptionByIDHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["id"]
		var sub Subscription
		err := db.QueryRow(r.Context(),
			`SELECT id, service_name, price, user_id, start_date, end_date
             FROM subscriptions WHERE id = $1`,
			id).Scan(&sub.ID, &sub.ServiceName, &sub.Price, &sub.UserID, &sub.StartDate, &sub.EndDate)
		if err != nil {
			log.Printf("GetSubscriptionByID: subscription not found: %s", id)
			http.Error(w, "subscription not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sub)
	}
}

// @Summary      Обновить подписку
// @Description  Обновляет поля существующей подписки по id
// @Tags         subscriptions
// @Accept       json
// @Produce      json
// @Param        id   path   string  true  "ID подписки"
// @Param        subscription  body  subscription.UpdateSubscriptionRequest  true  "Данные для обновления"
// @Success      204  {string}  string  "no content"
// @Failure      400  {string}  string  "invalid request"
// @Failure      404  {string}  string  "subscription not found"
// @Failure      500  {string}  string  "failed to update"
// @Router       /subscriptions/{id} [put]
func UpdateSubscriptionHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["id"]

		var req UpdateSubscriptionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		startDate, err := time.Parse("01-2006", req.StartDate)
		if err != nil {
			http.Error(w, "invalid start date", http.StatusBadRequest)
			return
		}
		var endDatePtr *time.Time
		if req.EndDate != nil {
			ed, err := time.Parse("01-2006", *req.EndDate)
			if err != nil {
				log.Printf("invalid end_date '%s': %v", *req.EndDate, err)
				http.Error(w, "invalid end date", http.StatusBadRequest)
				return
			}
			endDatePtr = &ed
		}
		_, err = db.Exec(r.Context(),
			`UPDATE subscriptions SET service_name = $1, price = $2, start_date = $3, end_date = $4 WHERE id = $5`,
			req.ServiceName, req.Price, startDate, endDatePtr, id)
		if err != nil {
			log.Printf("UpdateSubscription: failed for id %s: %v", id, err)
			http.Error(w, "failed to update", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// @Summary      Удалить подписку
// @Description  Удаляет подписку по id
// @Tags         subscriptions
// @Produce      json
// @Param        id   path   string  true  "ID подписки"
// @Success      204  {string}  string  "no content"
// @Failure      404  {string}  string  "subscription not found"
// @Failure      500  {string}  string  "db error"
// @Router       /subscriptions/{id} [delete]
func DeleteSubscriptionHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["id"]
		res, err := db.Exec(r.Context(),
			`DELETE FROM subscriptions WHERE id = $1`, id)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if res.RowsAffected() == 0 {
			http.Error(w, "subscription not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
