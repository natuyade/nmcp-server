package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type DelegateRequest struct {
	Role string `json:"role"`
	Task string `json:"task"`
}

type DelegateResponse struct {
	Result string `json:"result"`
}

func delegateHandler(
	w http.ResponseWriter,
	r *http.Request,
) {

	var request DelegateRequest

	// jsonデコード
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := DelegateResponse{
		Result: fmt.Sprintf(
			"role=%s task=%s",
			request.Role,
			request.Task,
		),
	}

	// jsonで返すことを明示
	w.Header().Set("Content-Type", "application/json")

	// jsonエンコード
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Println(err)
	}
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /delegate", delegateHandler)

	log.Println("dummy llm server started: http://127.0.0.1:8081")

	if err := http.ListenAndServe("127.0.0.1:8081", mux); err != nil {
		log.Fatal(err)
	}
}
