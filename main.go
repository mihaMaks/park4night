package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"park4night/internal/db"

	"github.com/xuri/excelize/v2"
	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	dbFile := "park4night.db"
	ctx := context.Background()

	// Open or create database
	database, err := sql.Open("sqlite", dbFile)
	if err != nil {
		log.Fatal("Error opening database:", err)
	}
	defer database.Close()

	// Create schema
	schemaSQL := `
	CREATE TABLE IF NOT EXISTS records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		date DATETIME,
		state TEXT,
		adults INTEGER,
		children INTEGER,
		bicycles INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := database.ExecContext(ctx, schemaSQL); err != nil {
		log.Fatal("Error creating schema:", err)
	}

	queries := db.New(database)

	switch os.Args[1] {
	case "prepare":
		prepareExcelFile()
	case "load":
		loadExcelData(ctx, queries)
	case "list":
		listRecords(ctx, queries)
	case "get":
		if len(os.Args) < 3 {
			fmt.Println("Usage: go run main.go get <id>")
			return
		}
		id, _ := strconv.ParseInt(os.Args[2], 10, 64)
		getRecord(ctx, queries, id)
	case "insert":
		insertRecord(ctx, queries)
	case "delete":
		if len(os.Args) < 3 {
			fmt.Println("Usage: go run main.go delete <id>")
			return
		}
		id, _ := strconv.ParseInt(os.Args[2], 10, 64)
		deleteRecord(ctx, queries, id)
	case "update":
		if len(os.Args) < 3 {
			fmt.Println("Usage: go run main.go update <id> <date> <state> <adults> <children> <bicycles>")
			return
		}
		id, _ := strconv.ParseInt(os.Args[2], 10, 64)
		updateRecord(ctx, queries, id)
	case "count":
		countRecords(ctx, queries)
	case "web":
		fmt.Println("Starting web server on http://localhost:8080")
		startWebServer(queries)
	default:
		fmt.Println("Unknown command:", os.Args[1])
		printUsage()
	}
}

func printUsage() {
	fmt.Println(`Usage:
  go run main.go prepare                                  - Create transformed_corrected.xlsx from transformed.xlsx
  go run main.go load                                     - Load data from transformed_corrected.xlsx
  go run main.go list [limit] [offset]                   - List all records (default limit=10, offset=0)
  go run main.go get <id>                                - Get a specific record
  go run main.go insert <date> <state> <adults> <children> <bicycles> - Insert a new record
  go run main.go delete <id>                             - Delete a record
  go run main.go update <id> <date> <state> <adults> <children> <bicycles> - Update a record
  go run main.go count                                    - Count total records
  go run main.go web                                      - Start web UI on http://localhost:8080`)

}

func prepareExcelFile() {
	fmt.Println("Preparing Excel file...")

	f, err := excelize.OpenFile("transformed.xlsx")
	if err != nil {
		log.Fatal("Error opening file:", err)
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		log.Fatal("Error reading rows:", err)
	}

	// Create a new workbook
	newFile := excelize.NewFile()
	newSheetName := "Sheet1"

	// Copy data to new file, skipping column A (index 0)
	for rowIdx, row := range rows {
		for colIdx := 1; colIdx < len(row); colIdx++ {
			// Convert to Excel cell reference (1-indexed for excelize)
			cell, err := excelize.CoordinatesToCellName(colIdx, rowIdx+1)
			if err != nil {
				log.Fatal("Error converting coordinates:", err)
			}
			newFile.SetCellValue(newSheetName, cell, row[colIdx])
		}
	}

	// Save the new file
	err = newFile.SaveAs("transformed_corrected.xlsx")
	if err != nil {
		log.Fatal("Error saving file:", err)
	}

	fmt.Println("Successfully created transformed_corrected.xlsx")
}

func loadExcelData(ctx context.Context, queries *db.Queries) {
	fmt.Println("Loading data from transformed_corrected.xlsx...")

	f, err := excelize.OpenFile("transformed_corrected.xlsx")
	if err != nil {
		log.Fatal("Error opening file:", err)
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		log.Fatal("Error reading rows:", err)
	}

	count := 0
	// Skip header row (row 0)
	for i := 1; i < len(rows); i++ {
		row := rows[i]

		// Get column values safely with bounds checking
		var dateStr, stateStr, adultsStr, childrenStr, bicyclesStr string

		if len(row) > 0 {
			dateStr = row[0]
		}
		if len(row) > 1 {
			stateStr = row[1]
		}
		if len(row) > 2 {
			adultsStr = row[2]
		}
		if len(row) > 3 {
			childrenStr = row[3]
		}
		if len(row) > 4 {
			bicyclesStr = row[4]
		}

		adults, _ := strconv.Atoi(adultsStr)
		children, _ := strconv.Atoi(childrenStr)
		bicycles, _ := strconv.Atoi(bicyclesStr)

		// Parse date - try multiple formats, extract only date part
		dateVal := sql.NullTime{}
		if dateStr != "" {
			var t time.Time
			var err error
			// Try parsing with time first (Excel format: 2023-03-14 00:00:00)
			if t, err = time.Parse("2006-01-02 15:04:05", dateStr); err != nil {
				// Fallback to date-only format
				t, err = time.Parse("2006-01-02", dateStr)
			}
			if err == nil {
				// Normalize to midnight (00:00:00)
				t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
				dateVal = sql.NullTime{Time: t, Valid: true}
			}
		}

		if err := queries.CreateRecord(ctx, db.CreateRecordParams{
			Date:     dateVal,
			State:    sql.NullString{String: stateStr, Valid: stateStr != ""},
			Adults:   sql.NullInt64{Int64: int64(adults), Valid: adultsStr != ""},
			Children: sql.NullInt64{Int64: int64(children), Valid: childrenStr != ""},
			Bicycles: sql.NullInt64{Int64: int64(bicycles), Valid: bicyclesStr != ""},
		}); err != nil {
			log.Printf("Error loading row %d: %v", i, err)
			continue
		}
		count++
	}

	fmt.Printf("Successfully loaded %d records into the database\n", count)
}

func listRecords(ctx context.Context, queries *db.Queries) {
	limit := int64(10)
	offset := int64(0)

	if len(os.Args) > 2 {
		l, _ := strconv.ParseInt(os.Args[2], 10, 64)
		limit = l
	}
	if len(os.Args) > 3 {
		o, _ := strconv.ParseInt(os.Args[3], 10, 64)
		offset = o
	}

	records, err := queries.ListRecords(ctx, db.ListRecordsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		log.Fatal("Error listing records:", err)
	}

	fmt.Printf("%-4s %-20s %-30s %-7s %-8s %-8s\n", "ID", "Date", "State", "Adults", "Children", "Bicycles")
	fmt.Println(strings.Repeat("-", 85))
	for _, record := range records {
		date := ""
		if record.Date.Valid {
			date = record.Date.Time.Format("2006-01-02")
		}
		state := ""
		if record.State.Valid {
			state = record.State.String
		}
		adults := ""
		if record.Adults.Valid {
			adults = strconv.FormatInt(record.Adults.Int64, 10)
		}
		children := ""
		if record.Children.Valid {
			children = strconv.FormatInt(record.Children.Int64, 10)
		}
		bicycles := ""
		if record.Bicycles.Valid {
			bicycles = strconv.FormatInt(record.Bicycles.Int64, 10)
		}

		fmt.Printf("%-4d %-20s %-30s %-7s %-8s %-8s\n",
			record.ID, date, state, adults, children, bicycles)
	}
}

func getRecord(ctx context.Context, queries *db.Queries, id int64) {
	record, err := queries.GetRecord(ctx, id)
	if err == sql.ErrNoRows {
		fmt.Printf("Record with ID %d not found\n", id)
		return
	}
	if err != nil {
		log.Fatal("Error getting record:", err)
	}

	fmt.Printf("ID: %d\n", record.ID)
	if record.Date.Valid {
		fmt.Printf("Date: %s\n", record.Date.Time.Format("2006-01-02"))
	}
	if record.State.Valid {
		fmt.Printf("State: %s\n", record.State.String)
	}
	if record.Adults.Valid {
		fmt.Printf("Adults: %d\n", record.Adults.Int64)
	}
	if record.Children.Valid {
		fmt.Printf("Children: %d\n", record.Children.Int64)
	}
	if record.Bicycles.Valid {
		fmt.Printf("Bicycles: %d\n", record.Bicycles.Int64)
	}
	if record.CreatedAt.Valid {
		fmt.Printf("Created at: %s\n", record.CreatedAt.Time.Format("2006-01-02 15:04:05"))
	}
	if record.UpdatedAt.Valid {
		fmt.Printf("Updated at: %s\n", record.UpdatedAt.Time.Format("2006-01-02 15:04:05"))
	}
}

func insertRecord(ctx context.Context, queries *db.Queries) {
	if len(os.Args) < 7 {
		fmt.Println("Usage: go run main.go insert <date> <state> <adults> <children> <bicycles>")
		return
	}

	adults, _ := strconv.ParseInt(os.Args[4], 10, 64)
	children, _ := strconv.ParseInt(os.Args[5], 10, 64)
	bicycles, _ := strconv.ParseInt(os.Args[6], 10, 64)

	// Parse date - try multiple formats, extract only date part
	dateVal := sql.NullTime{}
	if os.Args[2] != "" {
		var t time.Time
		var err error
		if t, err = time.Parse("2006-01-02 15:04:05", os.Args[2]); err != nil {
			t, err = time.Parse("2006-01-02", os.Args[2])
		}
		if err == nil {
			// Normalize to midnight (00:00:00)
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
			dateVal = sql.NullTime{Time: t, Valid: true}
		}
	}

	if err := queries.CreateRecord(ctx, db.CreateRecordParams{
		Date:     dateVal,
		State:    sql.NullString{String: os.Args[3], Valid: true},
		Adults:   sql.NullInt64{Int64: adults, Valid: true},
		Children: sql.NullInt64{Int64: children, Valid: true},
		Bicycles: sql.NullInt64{Int64: bicycles, Valid: true},
	}); err != nil {
		log.Fatal("Error inserting record:", err)
	}

	fmt.Println("Record inserted successfully")
}

func deleteRecord(ctx context.Context, queries *db.Queries, id int64) {
	if err := queries.DeleteRecord(ctx, id); err != nil {
		log.Fatal("Error deleting record:", err)
	}

	fmt.Printf("Record with ID %d deleted successfully\n", id)
}

func updateRecord(ctx context.Context, queries *db.Queries, id int64) {
	if len(os.Args) < 8 {
		fmt.Println("Usage: go run main.go update <id> <date> <state> <adults> <children> <bicycles>")
		return
	}

	adults, _ := strconv.ParseInt(os.Args[5], 10, 64)
	children, _ := strconv.ParseInt(os.Args[6], 10, 64)
	bicycles, _ := strconv.ParseInt(os.Args[7], 10, 64)

	// Parse date - try multiple formats, extract only date part
	dateVal := sql.NullTime{}
	if os.Args[3] != "" {
		var t time.Time
		var err error
		if t, err = time.Parse("2006-01-02 15:04:05", os.Args[3]); err != nil {
			t, err = time.Parse("2006-01-02", os.Args[3])
		}
		if err == nil {
			// Normalize to midnight (00:00:00)
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
			dateVal = sql.NullTime{Time: t, Valid: true}
		}
	}

	if err := queries.UpdateRecord(ctx, db.UpdateRecordParams{
		Date:     dateVal,
		State:    sql.NullString{String: os.Args[4], Valid: true},
		Adults:   sql.NullInt64{Int64: adults, Valid: true},
		Children: sql.NullInt64{Int64: children, Valid: true},
		Bicycles: sql.NullInt64{Int64: bicycles, Valid: true},
		ID:       id,
	}); err != nil {
		log.Fatal("Error updating record:", err)
	}

	fmt.Printf("Record with ID %d updated successfully\n", id)
}

func countRecords(ctx context.Context, queries *db.Queries) {
	count, err := queries.CountRecords(ctx)
	if err != nil {
		log.Fatal("Error counting records:", err)
	}
	fmt.Printf("Total records: %d\n", count)
}
