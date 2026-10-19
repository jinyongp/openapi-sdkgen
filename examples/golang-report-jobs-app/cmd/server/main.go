package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"example.com/sdkgen-go-report-jobs/generated/api/models"
)

type order struct {
	Date        string `json:"date"`
	Region      string `json:"region"`
	AmountCents int64  `json:"amountCents"`
}
type regionTotal struct {
	orders      int
	amountCents int64
}
type work struct {
	job     models.ReportJobOutput
	orders  []order
	regions map[string]regionTotal
	cancel  context.CancelFunc
	csv     string
}
type reports struct {
	mu     sync.Mutex
	orders []order
	jobs   map[string]*work
}

func (reports *reports) generate(ctx context.Context, work *work) {
	timer := time.NewTicker(200 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		reports.mu.Lock()
		if work.job.Status == "cancelled" {
			reports.mu.Unlock()
			return
		}
		work.job.Status = "running"
		if work.job.Processed < work.job.Total {
			item := work.orders[int(work.job.Processed)]
			total := work.regions[item.Region]
			total.orders++
			total.amountCents += item.AmountCents
			work.regions[item.Region] = total
			work.job.Processed++
		}
		if work.job.Processed == work.job.Total {
			regions := make([]string, 0, len(work.regions))
			for region := range work.regions {
				regions = append(regions, region)
			}
			sort.Strings(regions)
			var result strings.Builder
			result.WriteString("region,orders,revenueUSD\n")
			for _, region := range regions {
				total := work.regions[region]
				fmt.Fprintf(&result, "%s,%d,%d.%02d\n", region, total.orders, total.amountCents/100, total.amountCents%100)
			}
			work.csv = result.String()
			work.job.Status = "completed"
			reports.mu.Unlock()
			work.cancel()
			return
		}
		reports.mu.Unlock()
	}
}

func respond(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
func problem(w http.ResponseWriter, status int, message string) {
	respond(w, status, map[string]string{"message": message})
}

var monthPattern = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)
var regionPattern = regexp.MustCompile(`^[A-Z]+$`)

func (reports *reports) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" && r.URL.Path == "/reports" {
		var input struct {
			Month string `json:"month"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		if err := decoder.Decode(&input); err != nil || !monthPattern.MatchString(input.Month) {
			problem(w, 400, "Use a month such as 2026-10")
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			problem(w, 400, "Expected a single JSON month")
			return
		}
		selected := make([]order, 0)
		for _, item := range reports.orders {
			if strings.HasPrefix(item.Date, input.Month) {
				selected = append(selected, item)
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		work := &work{
			job:    models.ReportJobOutput{ID: rand.Text(), Month: input.Month, Status: "queued", Total: float64(len(selected))},
			orders: selected, regions: make(map[string]regionTotal), cancel: cancel,
		}
		reports.mu.Lock()
		reports.jobs[work.job.ID] = work
		job := work.job
		reports.mu.Unlock()
		go reports.generate(ctx, work)
		respond(w, 202, job)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/reports/"), "/")
	if !strings.HasPrefix(r.URL.Path, "/reports/") || len(parts) > 2 {
		problem(w, 404, "Route not found")
		return
	}
	reports.mu.Lock()
	work := reports.jobs[parts[0]]
	if work == nil {
		reports.mu.Unlock()
		problem(w, 404, "Report not found")
		return
	}
	if r.Method == "POST" && len(parts) == 2 && parts[1] == "cancel" {
		if work.job.Status == "completed" {
			reports.mu.Unlock()
			problem(w, 409, "Completed reports cannot be cancelled")
			return
		}
		work.job.Status = "cancelled"
		work.cancel()
	}
	job, csv := work.job, work.csv
	reports.mu.Unlock()
	if r.Method == "GET" && len(parts) == 1 {
		respond(w, 200, job)
		return
	}
	if r.Method == "POST" && len(parts) == 2 && parts[1] == "cancel" {
		respond(w, 200, job)
		return
	}
	if r.Method == "GET" && len(parts) == 2 && parts[1] == "file" {
		if job.Status != "completed" {
			problem(w, 409, "Report is not completed")
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="monthly-sales.csv"`)
		_, _ = io.WriteString(w, csv)
		return
	}
	problem(w, 404, "Route not found")
}

func main() {
	data, err := os.ReadFile("orders.json")
	if err != nil {
		log.Fatal(err)
	}
	var orders []order
	if err := json.Unmarshal(data, &orders); err != nil {
		log.Fatal(err)
	}
	for _, item := range orders {
		if _, err := time.Parse("2006-01-02", item.Date); err != nil || !regionPattern.MatchString(item.Region) || item.AmountCents < 0 {
			log.Fatal("orders.json must contain dated orders with region and non-negative integer cents")
		}
	}
	port := os.Getenv("REPORT_API_PORT")
	if port == "" {
		port = "18796"
	}
	server := &http.Server{Addr: "127.0.0.1:" + port, Handler: &reports{orders: orders, jobs: make(map[string]*work)}, ReadHeaderTimeout: 5 * time.Second}
	log.Print("Report API listening on ", server.Addr)
	log.Fatal(server.ListenAndServe())
}
