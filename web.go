package main

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"park4night/internal/db"

	"github.com/gin-gonic/gin"
)

type RecordResponse struct {
	ID       int64  `json:"id"`
	Date     string `json:"date"`
	State    string `json:"state"`
	Adults   int64  `json:"adults"`
	Children int64  `json:"children"`
	Bicycles int64  `json:"bicycles"`
}

type InsertRecordRequest struct {
	Date     string `json:"date" binding:"required"`
	State    string `json:"state" binding:"required"`
	Adults   int64  `json:"adults"`
	Children int64  `json:"children"`
	Bicycles int64  `json:"bicycles"`
}

type UploadSummary struct {
	TotalRecords  int
	TotalAdults   int64
	TotalChildren int64
	TotalBicycles int64
	UniqueStates  int
	SampleRecords []RecordResponse
	FileId        string
}

func startWebServer(queries *db.Queries) {
	router := gin.Default()

	// Serve static files
	router.Static("/static", "./static")

	// Serve HTML pages
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})
	router.GET("/upload", func(c *gin.Context) {
		c.HTML(http.StatusOK, "upload.html", nil)
	})
	router.POST("/upload", func(c *gin.Context) {
		webUploadExcel(c, queries)
	})
	router.POST("/integrate", func(c *gin.Context) {
		webIntegrateData(c, queries)
	})

	// API Routes
	api := router.Group("/api")
	{
		api.GET("/records", func(c *gin.Context) {
			webListRecords(c, queries)
		})
		api.POST("/records", func(c *gin.Context) {
			webCreateRecord(c, queries)
		})
		api.GET("/records/:id", func(c *gin.Context) {
			webGetRecord(c, queries)
		})
		api.PUT("/records/:id", func(c *gin.Context) {
			webUpdateRecord(c, queries)
		})
		api.DELETE("/records/:id", func(c *gin.Context) {
			webDeleteRecord(c, queries)
		})
		api.GET("/chart-data", func(c *gin.Context) {
			webGetChartData(c, queries)
		})
		api.GET("/states", func(c *gin.Context) {
			webGetStates(c, queries)
		})
		api.GET("/forecast", func(c *gin.Context) {
			webGetForecast(c, queries)
		})
	}

	router.LoadHTMLGlob("templates/*.html")
	router.Run(":9090")
}

func webListRecords(c *gin.Context, queries *db.Queries) {
	ctx := c.Request.Context()

	// Get filter parameters
	dateFilter := c.Query("date")
	stateFilter := c.Query("state")
	minAdultsStr := c.Query("minAdults")
	minChildrenStr := c.Query("minChildren")
	minBicyclesStr := c.Query("minBicycles")

	limit := int64(50)
	offset := int64(0)

	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.ParseInt(l, 10, 64); err == nil {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.ParseInt(o, 10, 64); err == nil {
			offset = parsed
		}
	}

	// Convert string filters to appropriate types
	var dateFilterParam interface{} = nil
	if dateFilter != "" {
		dateFilterParam = dateFilter
	}

	var stateFilterParam interface{} = nil
	if stateFilter != "" {
		stateFilterParam = stateFilter
	}

	var minAdultsParam interface{} = nil
	if minAdultsStr != "" {
		if val, err := strconv.ParseInt(minAdultsStr, 10, 64); err == nil && val > 0 {
			minAdultsParam = val
		}
	}

	var minChildrenParam interface{} = nil
	if minChildrenStr != "" {
		if val, err := strconv.ParseInt(minChildrenStr, 10, 64); err == nil && val > 0 {
			minChildrenParam = val
		}
	}

	var minBicyclesParam interface{} = nil
	if minBicyclesStr != "" {
		if val, err := strconv.ParseInt(minBicyclesStr, 10, 64); err == nil && val > 0 {
			minBicyclesParam = val
		}
	}

	// Get total count with filters
	totalCount, err := queries.CountRecordsWithFilters(ctx, db.CountRecordsWithFiltersParams{
		DateFilter:  dateFilterParam,
		StateFilter: stateFilterParam,
		MinAdults:   minAdultsParam,
		MinChildren: minChildrenParam,
		MinBicycles: minBicyclesParam,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Fetch paginated results with filters
	records, err := queries.ListRecordsWithFilters(ctx, db.ListRecordsWithFiltersParams{
		DateFilter:  dateFilterParam,
		StateFilter: stateFilterParam,
		MinAdults:   minAdultsParam,
		MinChildren: minChildrenParam,
		MinBicycles: minBicyclesParam,
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to response format
	var response []RecordResponse
	for _, record := range records {
		date := ""
		if record.Date.Valid {
			date = record.Date.Time.Format("2006-01-02")
		}
		state := ""
		if record.State.Valid {
			state = record.State.String
		}
		adults := int64(0)
		if record.Adults.Valid {
			adults = record.Adults.Int64
		}
		children := int64(0)
		if record.Children.Valid {
			children = record.Children.Int64
		}
		bicycles := int64(0)
		if record.Bicycles.Valid {
			bicycles = record.Bicycles.Int64
		}

		response = append(response, RecordResponse{
			ID:       record.ID,
			Date:     date,
			State:    state,
			Adults:   adults,
			Children: children,
			Bicycles: bicycles,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  response,
		"total": totalCount,
	})
}

func webCreateRecord(c *gin.Context, queries *db.Queries) {
	ctx := context.Background()

	var req InsertRecordRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse date - try multiple formats, extract only date part
	dateVal := sql.NullTime{}
	if req.Date != "" {
		var t time.Time
		var err error
		if t, err = time.Parse("2006-01-02 15:04:05", req.Date); err != nil {
			t, err = time.Parse("2006-01-02", req.Date)
		}
		if err == nil {
			// Normalize to midnight (00:00:00)
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
			dateVal = sql.NullTime{Time: t, Valid: true}
		}
	}

	if err := queries.CreateRecord(ctx, db.CreateRecordParams{
		Date:     dateVal,
		State:    sql.NullString{String: req.State, Valid: req.State != ""},
		Adults:   sql.NullInt64{Int64: req.Adults, Valid: true},
		Children: sql.NullInt64{Int64: req.Children, Valid: true},
		Bicycles: sql.NullInt64{Int64: req.Bicycles, Valid: true},
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Record created successfully"})
}

func webGetRecord(c *gin.Context, queries *db.Queries) {
	ctx := context.Background()

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	record, err := queries.GetRecord(ctx, id)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	date := ""
	if record.Date.Valid {
		date = record.Date.Time.Format("2006-01-02")
	}
	state := ""
	if record.State.Valid {
		state = record.State.String
	}

	c.JSON(http.StatusOK, RecordResponse{
		ID:       record.ID,
		Date:     date,
		State:    state,
		Adults:   record.Adults.Int64,
		Children: record.Children.Int64,
		Bicycles: record.Bicycles.Int64,
	})
}

func webUpdateRecord(c *gin.Context, queries *db.Queries) {
	ctx := context.Background()

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var req InsertRecordRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse date - try multiple formats, extract only date part
	dateVal := sql.NullTime{}
	if req.Date != "" {
		var t time.Time
		var err error
		if t, err = time.Parse("2006-01-02 15:04:05", req.Date); err != nil {
			t, err = time.Parse("2006-01-02", req.Date)
		}
		if err == nil {
			// Normalize to midnight (00:00:00)
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
			dateVal = sql.NullTime{Time: t, Valid: true}
		}
	}

	if err := queries.UpdateRecord(ctx, db.UpdateRecordParams{
		Date:     dateVal,
		State:    sql.NullString{String: req.State, Valid: req.State != ""},
		Adults:   sql.NullInt64{Int64: req.Adults, Valid: true},
		Children: sql.NullInt64{Int64: req.Children, Valid: true},
		Bicycles: sql.NullInt64{Int64: req.Bicycles, Valid: true},
		ID:       id,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Record updated successfully"})
}

func webDeleteRecord(c *gin.Context, queries *db.Queries) {
	ctx := context.Background()

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	if err := queries.DeleteRecord(ctx, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Record deleted successfully"})
}

func webGetChartData(c *gin.Context, queries *db.Queries) {
	ctx := c.Request.Context()

	// Get filter parameters
	dateFrom := c.Query("dateFrom")
	dateTo := c.Query("dateTo")
	minChildrenStr := c.Query("minChildren")
	minBicyclesStr := c.Query("minBicycles")
	stateFilter := c.Query("state") // Still read state param for state-specific requests from JS

	// Parse dates
	var dateFromTime, dateToTime time.Time
	if dateFrom != "" {
		if parsed, err := time.Parse("2006-01-02", dateFrom); err == nil {
			dateFromTime = parsed
		}
	}
	if dateTo != "" {
		if parsed, err := time.Parse("2006-01-02", dateTo); err == nil {
			dateToTime = parsed.AddDate(0, 0, 1) // Include entire day
		}
	}

	// Convert minChildren and minBicycles to int64
	var minChildrenVal int64
	var minBicyclesVal int64

	if minChildrenStr != "" {
		if parsed, err := strconv.ParseInt(minChildrenStr, 10, 64); err == nil && parsed > 0 {
			minChildrenVal = parsed
		}
	}
	if minBicyclesStr != "" {
		if parsed, err := strconv.ParseInt(minBicyclesStr, 10, 64); err == nil && parsed > 0 {
			minBicyclesVal = parsed
		}
	}

	// Prepare query parameters
	params := db.ListRecordsForChartParams{
		Column1:  nil,
		Date:     sql.NullTime{Valid: false},
		Column3:  nil,
		Date_2:   sql.NullTime{Valid: false},
		Column5:  nil,
		Children: sql.NullInt64{Valid: false},
		Column7:  nil,
		Bicycles: sql.NullInt64{Valid: false},
	}

	// Set date filter if provided
	if !dateFromTime.IsZero() {
		params.Column1 = true
		params.Date = sql.NullTime{Time: dateFromTime, Valid: true}
	}
	if !dateToTime.IsZero() {
		params.Column3 = true
		params.Date_2 = sql.NullTime{Time: dateToTime, Valid: true}
	}

	// Set minChildren filter if provided
	if minChildrenVal > 0 {
		params.Column5 = true
		params.Children = sql.NullInt64{Int64: minChildrenVal, Valid: true}
	}

	// Set minBicycles filter if provided
	if minBicyclesVal > 0 {
		params.Column7 = true
		params.Bicycles = sql.NullInt64{Int64: minBicyclesVal, Valid: true}
	}

	// Fetch records
	records, err := queries.ListRecordsForChart(ctx, params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Group by date and build chart data structure
	dateCounts := make(map[string]int)
	tooltipsByDate := make(map[string]map[string]map[string]interface{})

	for _, record := range records {
		if !record.Date.Valid {
			continue
		}

		recordDate := record.Date.Time.Format("2006-01-02")

		// Filter by state if provided (for state-specific requests)
		if stateFilter != "" && (!record.State.Valid || record.State.String != stateFilter) {
			continue
		}

		// Increment count for this date
		dateCounts[recordDate]++

		// Initialize tooltip map for this date if needed
		if tooltipsByDate[recordDate] == nil {
			tooltipsByDate[recordDate] = make(map[string]map[string]interface{})
		}

		// Get or create state entry
		state := "Unknown"
		if record.State.Valid && record.State.String != "" {
			state = record.State.String
		}

		if tooltipsByDate[recordDate][state] == nil {
			tooltipsByDate[recordDate][state] = map[string]interface{}{
				"count":    0,
				"adults":   int64(0),
				"children": int64(0),
				"bicycles": int64(0),
			}
		}

		// Aggregate the values
		tooltipsByDate[recordDate][state]["count"] = tooltipsByDate[recordDate][state]["count"].(int) + 1
		adults := int64(0)
		if record.Adults.Valid {
			adults = record.Adults.Int64
		}
		children := int64(0)
		if record.Children.Valid {
			children = record.Children.Int64
		}
		bicycles := int64(0)
		if record.Bicycles.Valid {
			bicycles = record.Bicycles.Int64
		}
		tooltipsByDate[recordDate][state]["adults"] = tooltipsByDate[recordDate][state]["adults"].(int64) + adults
		tooltipsByDate[recordDate][state]["children"] = tooltipsByDate[recordDate][state]["children"].(int64) + children
		tooltipsByDate[recordDate][state]["bicycles"] = tooltipsByDate[recordDate][state]["bicycles"].(int64) + bicycles
	}

	// Convert to sorted array for chart
	type ChartPoint struct {
		Date  string `json:"date"`
		Count int    `json:"count"`
	}

	var chartData []ChartPoint
	for date, count := range dateCounts {
		chartData = append(chartData, ChartPoint{Date: date, Count: count})
	}

	// Sort by date (oldest first)
	for i := 0; i < len(chartData)-1; i++ {
		for j := i + 1; j < len(chartData); j++ {
			if chartData[i].Date > chartData[j].Date {
				chartData[i], chartData[j] = chartData[j], chartData[i]
			}
		}
	}

	// Format tooltips
	type TooltipDetail struct {
		State    string `json:"state"`
		Count    int    `json:"count"`
		Adults   int64  `json:"adults"`
		Children int64  `json:"children"`
		Bicycles int64  `json:"bicycles"`
	}

	tooltips := make(map[string][]TooltipDetail)
	for date, stateMap := range tooltipsByDate {
		var details []TooltipDetail
		for state, values := range stateMap {
			details = append(details, TooltipDetail{
				State:    state,
				Count:    values["count"].(int),
				Adults:   values["adults"].(int64),
				Children: values["children"].(int64),
				Bicycles: values["bicycles"].(int64),
			})
		}
		// Sort by state name for consistent ordering
		for i := 0; i < len(details)-1; i++ {
			for j := i + 1; j < len(details); j++ {
				if details[i].State > details[j].State {
					details[i], details[j] = details[j], details[i]
				}
			}
		}
		tooltips[date] = details
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     chartData,
		"tooltips": tooltips,
	})
}

func webGetStates(c *gin.Context, queries *db.Queries) {
	ctx := context.Background()

	// Get all records
	records, err := queries.ListRecords(ctx, db.ListRecordsParams{
		Limit:  999999, // Get all
		Offset: 0,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Collect unique states
	stateSet := make(map[string]bool)
	for _, record := range records {
		if record.State.Valid && record.State.String != "" {
			stateSet[record.State.String] = true
		}
	}

	// Convert to sorted array
	var states []string
	for state := range stateSet {
		states = append(states, state)
	}

	// Sort states alphabetically
	for i := 0; i < len(states)-1; i++ {
		for j := i + 1; j < len(states); j++ {
			if states[i] > states[j] {
				states[i], states[j] = states[j], states[i]
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"states": states})
}

func webGetForecast(c *gin.Context, queries *db.Queries) {
	ctx := context.Background()

	// Determine the day-of-year targets (today, tomorrow, +2) using date-only calculations
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrow := today.AddDate(0, 0, 1)
	in2Days := today.AddDate(0, 0, 2)

	labels := []string{
		"Today (" + today.Format("02.01") + ")",
		"Tomorrow (" + tomorrow.Format("02.01") + ")",
		"In 2 days (" + in2Days.Format("02.01") + ")",
	}
	targets := []string{
		today.Format("01-02"),
		tomorrow.Format("01-02"),
		in2Days.Format("01-02"),
	}

	records, err := queries.ListRecords(ctx, db.ListRecordsParams{
		Limit:  999999,
		Offset: 0,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Determine the last 3 available years in the data
	yearSet := make(map[int]struct{})
	for _, record := range records {
		if !record.Date.Valid {
			continue
		}
		yearSet[record.Date.Time.Year()] = struct{}{}
	}

	var years []int
	for y := range yearSet {
		years = append(years, y)
	}
	// Sort descending (newest first)
	for i := 0; i < len(years)-1; i++ {
		for j := i + 1; j < len(years); j++ {
			if years[i] < years[j] {
				years[i], years[j] = years[j], years[i]
			}
		}
	}
	if len(years) > 3 {
		years = years[:3]
	}

	// Initialize counts for each year
	counts := make(map[int][]int)
	for _, y := range years {
		counts[y] = []int{0, 0, 0}
	}

	// Tally records for the target month/day in each year
	for _, record := range records {
		if !record.Date.Valid {
			continue
		}

		year := record.Date.Time.Year()
		if _, ok := counts[year]; !ok {
			continue
		}

		monthDay := record.Date.Time.Format("01-02")
		for idx, target := range targets {
			if monthDay == target {
				counts[year][idx]++
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"years":  years,
		"labels": labels,
		"counts": counts,
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && s[:len(substr)] == substr))
}

func webUploadExcel(c *gin.Context, queries *db.Queries) {
	file, header, err := c.Request.FormFile("excelFile")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}
	defer file.Close()

	tempDir := "./uploads"
	os.MkdirAll(tempDir, 0755)
	tempFile := filepath.Join(tempDir, header.Filename)
	out, err := os.Create(tempFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}
	defer out.Close()
	io.Copy(out, file)

	transformedData, err := processExcelFile(tempFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	transformedFile := strings.TrimSuffix(tempFile, filepath.Ext(tempFile)) + "_transformed.xlsx"
	err = saveTransformedExcel(transformedData, transformedFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save transformed file"})
		return
	}

	summary := generateSummary(transformedData)
	fileId := filepath.Base(transformedFile)
	summary.FileId = fileId

	c.HTML(http.StatusOK, "summary.html", summary)
}

func webIntegrateData(c *gin.Context, queries *db.Queries) {
	fileId := c.PostForm("fileId")
	if fileId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file ID provided"})
		return
	}

	transformedFile := filepath.Join("./uploads", fileId)
	data, err := loadTransformedData(transformedFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	for _, record := range data {
		dateVal := sql.NullTime{}
		if record.Date != "" {
			var t time.Time
			var err error
			if t, err = time.Parse("2006-01-02 15:04:05", record.Date); err != nil {
				t, err = time.Parse("2006-01-02", record.Date)
			}
			if err == nil {
				// Normalize to midnight (00:00:00)
				t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
				dateVal = sql.NullTime{Time: t, Valid: true}
			}
		}

		err := queries.CreateRecord(ctx, db.CreateRecordParams{
			Date:     dateVal,
			State:    sql.NullString{String: record.State, Valid: record.State != ""},
			Adults:   sql.NullInt64{Int64: record.Adults, Valid: true},
			Children: sql.NullInt64{Int64: record.Children, Valid: true},
			Bicycles: sql.NullInt64{Int64: record.Bicycles, Valid: true},
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	os.Remove(transformedFile)
	os.Remove(strings.TrimSuffix(transformedFile, "_transformed.xlsx") + filepath.Ext(transformedFile))

	c.Redirect(http.StatusFound, "/")
}
