package main

import (
	"log"
	"net/http"
	"strings"
	"time"
	"fmt"

	"database/sql"

	_ "github.com/go-sql-driver/mysql"
	"os"
)

type eventlog struct {
	At    time.Time
	Name  string
	Value string
}


func bulkInsert(q chan eventlog, db *sql.DB) {
	valueStrings := []string{}
	valueArgs := [](interface{}){}
	for {
		select {
		case eventlog, ok := <-q:
			if ok {
				valueStrings = append(valueStrings, "(?, ?, ?)")
				valueArgs = append(valueArgs, eventlog.At)
				valueArgs = append(valueArgs, eventlog.Name)
				valueArgs = append(valueArgs, eventlog.Value)
			} else {
				panic("channel is closed")
			}
		case <-time.After(1 * time.Second):
			if len(valueStrings) == 0 {
				continue
			}
			// insert
			stmt := fmt.Sprintf("INSERT INTO eventlog(at, name, value) VALUES %s", strings.Join(valueStrings, ","))
			_, e := db.Exec(stmt, valueArgs...)
			if e != nil {
				panic(e.Error())
			}
			valueStrings = []string{}
			valueArgs = [](interface{}){}
		}
	}

}
func main() {
	dataSourceName := os.Getenv("HAKARU_DATASOURCENAME")
	if dataSourceName == "" {
		dataSourceName = "root:password@tcp(127.0.0.1:13306)/hakaru"
	}
	db, err := sql.Open("mysql", dataSourceName)
	if err != nil {
		panic(err.Error())
	}
	db.SetMaxOpenConns(40)
	defer db.Close()

	q := make(chan eventlog, 100000)
	jst, e := time.LoadLocation("Asia/Tokyo")
	if e != nil {
		panic(e.Error())
	}

	bulkInsert(q, db)

	hakaruHandler := func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		value := r.URL.Query().Get("value")

		q <- eventlog{Name: name, Value: value, At: time.Now().In(jst)}

		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET")
	}

	http.HandleFunc("/hakaru", hakaruHandler)
	http.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	// start server
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatal(err)
	}
}
