package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/mux"
)

type Item struct {
	ShortDescription string `json:"shortDescription"`
	Price            string `json:"price"`
}

type Receipt struct {
	Retailer     string `json:"retailer"`
	PurchaseDate string `json:"purchaseDate"`
	PurchaseTime string `json:"purchaseTime"`
	Items        []Item `json:"items"`
	Total        string `json:"total"`
}

type ReceiptResponse struct {
	ID string `json:"id"`
}

type PointsResponse struct {
	Points int64 `json:"points"`
}

var (
	receipts = make(map[string]Receipt)
	mu       sync.Mutex
	idRegex  = regexp.MustCompile(`^\S+$`)
)

func processReceiptHandler(w http.ResponseWriter, r *http.Request) {
	var receipt Receipt
	if err := json.NewDecoder(r.Body).Decode(&receipt); err != nil {
		http.Error(w, "Invalid receipt", http.StatusBadRequest)
		return
	}

	id := fmt.Sprintf("%d", len(receipts)+1) // Simple ID generation
	mu.Lock()
	receipts[id] = receipt
	mu.Unlock()

	response := ReceiptResponse{ID: id}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func getPointsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if !idRegex.MatchString(id) {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	mu.Lock()
	receipt, exists := receipts[id]
	mu.Unlock()

	if !exists {
		http.Error(w, "No receipt found for that id", http.StatusNotFound)
		return
	}

	points := calculatePoints(receipt)
	response := PointsResponse{Points: points}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func calculatePoints(receipt Receipt) int64 {
	var points int64

	// One point for every alphanumeric character in the retailer name.
	retailer := receipt.Retailer
	var totalCharacters int64
	for _, char := range retailer {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') {
			totalCharacters++
		}
	}
	fmt.Println(totalCharacters, "points - Retailer name has", totalCharacters, "characters")
	points += totalCharacters

	totalFloat, err := strconv.ParseFloat(receipt.Total, 64)
	if err != nil {
		log.Printf("Error parsing total: %v", err)
		// 50 points if the total is a round dollar amount with no cents.
	} else if totalFloat == float64(int(totalFloat)) {
		fmt.Println("50 points - total is a round number!")
		points += 50
	}

	// 25 points if the total is a multiple of 0.25.
	if int(totalFloat*100)%25 == 0 && (float64(int(totalFloat*100)) == totalFloat*100) {
		fmt.Println("25 points - total is a multiple of 0.25!")
		points += 25
	}

	// Extract the number of items in the receipt
	numberOfItems := len(receipt.Items)
	// Count how many groups of two are in the number of items
	groupsOfTwo := numberOfItems / 2
	// 5 points for every two items on the receipt.
	fmt.Println(int64(groupsOfTwo*5), " points - (", groupsOfTwo, "pairs @ 5 points each)")
	points += int64(groupsOfTwo * 5)

	// If the trimmed length of the item description is a multiple of 3,
	// multiply the price by 0.2 and round up to the nearest integer.
	// The result is the number of points earned
	for _, item := range receipt.Items {
		if len(strings.TrimSpace(item.ShortDescription))%3 == 0 {
			price, err := strconv.ParseFloat(item.Price, 64)
			if err != nil {
				log.Printf("Error parsing price: %v", err)
			} else {
				points += int64(math.Ceil(price * 0.2))
				fmt.Println(int64(math.Ceil(price*0.2)), " points - ", strings.TrimSpace(item.ShortDescription), " is ",
					len(strings.TrimSpace(item.ShortDescription)),
					" characters (a multiple of 3) item price of ", price, " * 0.2 = ", price*0.2,
					"rounded up is ", int64(math.Ceil(price*0.2)), " points")
			}
		}
	}

	// 6 points if the day in the purchase date is odd.
	day, err := strconv.Atoi(receipt.PurchaseDate[8:])
	if err != nil {
		log.Printf("Error parsing day: %v", err)
	} else if day%2 != 0 {
		fmt.Println("6 points - purchase day is ", day, " which is odd")
		points += 6
	}

	// 10 points if the time of purchase is after 2:00pm and before 4:00pm.
	hour, err_hr := strconv.Atoi(receipt.PurchaseTime[:2])
	minute, err_min := strconv.Atoi(receipt.PurchaseTime[3:])
	if err_hr != nil || err_min != nil {
		log.Printf("Error parsing hour or minutes: %v", err)
	} else if (hour > 14 || (hour == 14 && minute > 0)) && (hour < 16) {
		fmt.Println("10 points - purchase time is ", hour, ":", minute, " which is after 2:00pm and before 4:00pm")
		points += 10
	}

	fmt.Println(points, " points")
	return points
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/receipts/process", processReceiptHandler).Methods("POST")
	r.HandleFunc("/receipts/{id}/points", getPointsHandler).Methods("GET")

	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("Could not start server: %s\n", err.Error())
	}
}
