package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type TransformedRecord struct {
	Date     string
	State    string
	Adults   int64
	Children int64
	Bicycles int64
}

func convertXlsToXlsx(xlsPath, xlsxPath string) error {
	// Use Python to convert .xls to .xlsx
	cmd := exec.Command("/Users/maksbertoncelj/Documents/park4night/.venv/bin/python", "-c", fmt.Sprintf(`
import pandas as pd
import sys
df = pd.read_excel('%s')
df.to_excel('%s', index=False)
`, xlsPath, xlsxPath))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("conversion failed: %v, output: %s", err, string(output))
	}
	return nil
}

func processExcelFile(filename string) ([]TransformedRecord, error) {
	// Check if file is .xls and convert to .xlsx if needed
	if strings.ToLower(filepath.Ext(filename)) == ".xls" {
		xlsxFilename := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".xlsx"
		err := convertXlsToXlsx(filename, xlsxFilename)
		if err != nil {
			return nil, fmt.Errorf("failed to convert .xls to .xlsx: %v", err)
		}
		filename = xlsxFilename
		// Clean up the temporary .xlsx file after processing
		defer os.Remove(filename)
	}

	f, err := excelize.OpenFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file %s: %v", filename, err)
	}
	defer f.Close()

	sheetName := f.GetSheetList()[0]
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to read rows from sheet %s: %v", sheetName, err)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("no data in sheet")
	}

	headers := rows[0]
	dataRows := rows[1:]

	// Limit to first 25 columns, exactly as in main.py
	maxCol := 25
	if len(headers) < maxCol {
		maxCol = len(headers)
	}

	// Melt
	melted := make([]map[string]string, 0)
	for _, row := range dataRows {
		if len(row) == 0 {
			continue
		}

		datum := strings.TrimSpace(row[0])
		if datum == "" {
			continue
		}

		for i := 1; i < maxCol && i < len(row); i++ {
			variable := strings.TrimSpace(headers[i])
			value := strings.TrimSpace(row[i])
			if value == "" || variable == "" {
				continue
			}
			melted = append(melted, map[string]string{
				"DATUM":    datum,
				"Variable": variable,
				"Value":    value,
			})
		}
	}

	// Split variable into base + group
	for _, item := range melted {
		variable := item["Variable"]
		if variable == "" {
			continue
		}
		parts := strings.SplitN(variable, ".", 2)
		item["Variable"] = parts[0]
		if len(parts) > 1 {
			item["Group"] = parts[1]
		} else {
			item["Group"] = "0"
		}
	}

	// Pivot (DATUM + Group -> variables)
	grouped := make(map[string]map[string]map[string]string)
	for _, item := range melted {
		datum := item["DATUM"]
		group := item["Group"]
		variable := item["Variable"]
		value := item["Value"]

		if _, ok := grouped[datum]; !ok {
			grouped[datum] = make(map[string]map[string]string)
		}
		if _, ok := grouped[datum][group]; !ok {
			grouped[datum][group] = make(map[string]string)
		}
		// If same cell appears multiple times, keep first (like aggfunc='first')
		if _, exists := grouped[datum][group][variable]; !exists {
			grouped[datum][group][variable] = value
		}
	}

	// Build final record list from grouped data
	records := make([]TransformedRecord, 0, 512)
	for datum, groups := range grouped {
		for _, vars := range groups {
			state := vars["DRŽAVA"]
			if state == "" {
				state = vars["DRZAVA"]
			}

			// parse numeric fields with fallback 0
			adults := int64(0)
			if v, ok := vars["ODRASLI"]; ok && v != "" {
				if f, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64); err == nil {
					adults = int64(f)
				}
			}
			children := int64(0)
			if v, ok := vars["OTROCI"]; ok && v != "" {
				if f, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64); err == nil {
					children = int64(f)
				}
			}
			bicycles := int64(0)
			if v, ok := vars["KOLESARJI"]; ok && v != "" {
				if f, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64); err == nil {
					bicycles = int64(f)
				}
			} else if v, ok := vars["KOLO"]; ok && v != "" {
				if f, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64); err == nil {
					bicycles = int64(f)
				}
			}

			records = append(records, TransformedRecord{
				Date:     datum,
				State:    state,
				Adults:   adults,
				Children: children,
				Bicycles: bicycles,
			})
		}
	}

	// Sort by date
	sort.Slice(records, func(i, j int) bool {
		return records[i].Date < records[j].Date
	})

	return records, nil
}

func saveTransformedExcel(records []TransformedRecord, filename string) error {
	f := excelize.NewFile()
	sheetName := "Sheet1"

	// Set headers
	f.SetCellValue(sheetName, "A1", "DATUM")
	f.SetCellValue(sheetName, "B1", "DRŽAVA")
	f.SetCellValue(sheetName, "C1", "ODRASLI")
	f.SetCellValue(sheetName, "D1", "OTROCI")
	f.SetCellValue(sheetName, "E1", "KOLESARJI")

	// Set data
	for i, record := range records {
		row := i + 2
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), record.Date)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), record.State)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), record.Adults)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), record.Children)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), record.Bicycles)
	}

	return f.SaveAs(filename)
}

func generateSummary(records []TransformedRecord) UploadSummary {
	summary := UploadSummary{
		TotalRecords: len(records),
	}

	stateSet := make(map[string]bool)

	for _, record := range records {
		summary.TotalAdults += record.Adults
		summary.TotalChildren += record.Children
		summary.TotalBicycles += record.Bicycles
		stateSet[record.State] = true
	}

	summary.UniqueStates = len(stateSet)

	// Sample records (first 5)
	sampleCount := 5
	if len(records) < sampleCount {
		sampleCount = len(records)
	}
	for i := 0; i < sampleCount; i++ {
		summary.SampleRecords = append(summary.SampleRecords, RecordResponse{
			Date:     records[i].Date,
			State:    records[i].State,
			Adults:   records[i].Adults,
			Children: records[i].Children,
			Bicycles: records[i].Bicycles,
		})
	}

	return summary
}

func loadTransformedData(filename string) ([]TransformedRecord, error) {
	f, err := excelize.OpenFile(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sheetName := f.GetSheetList()[0]
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, err
	}

	var records []TransformedRecord
	for i, row := range rows {
		if i == 0 { // Skip header
			continue
		}
		if len(row) < 5 {
			continue
		}

		parseNumber := func(val string) int64 {
			if val == "" {
				return 0
			}
			val = strings.ReplaceAll(val, ",", ".")
			if iv, err := strconv.ParseInt(val, 10, 64); err == nil {
				return iv
			}
			if fv, err := strconv.ParseFloat(val, 64); err == nil {
				return int64(fv)
			}
			return 0
		}

		adults := parseNumber(row[2])
		children := parseNumber(row[3])
		bicycles := parseNumber(row[4])

		record := TransformedRecord{
			Date:     row[0],
			State:    row[1],
			Adults:   adults,
			Children: children,
			Bicycles: bicycles,
		}
		records = append(records, record)
	}

	return records, nil
}
