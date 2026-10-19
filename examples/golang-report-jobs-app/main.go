package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	api "example.com/sdkgen-go-report-jobs/generated/api"
)

func run(ctx context.Context) error {
	baseURL := os.Getenv("REPORT_API_BASE_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:18796"
	}
	client, err := api.NewClient(api.ClientOptions{BaseURL: baseURL})
	if err != nil {
		return err
	}
	job, err := client.Operations.CreateReport(api.CreateReportInput{Body: api.CreateReportBody{Month: "2026-10"}}).Do(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Report: %s (%g/%g)\n", job.Status, job.Processed, job.Total)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for job.Status == "queued" || job.Status == "running" {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
		next, err := client.Operations.GetReport(api.GetReportInput{Path: api.GetReportPath{ReportID: job.ID}}).Do(ctx)
		if err != nil {
			return err
		}
		if next.Status != job.Status || next.Processed != job.Processed {
			fmt.Printf("Report: %s (%g/%g)\n", next.Status, next.Processed, next.Total)
		}
		job = next
	}
	if job.Status != "completed" {
		return fmt.Errorf("report ended as %s", job.Status)
	}
	file, err := client.Operations.DownloadReport(api.DownloadReportInput{Path: api.DownloadReportPath{ReportID: job.ID}}).Do(ctx)
	if err != nil {
		return err
	}
	if err := os.WriteFile("monthly-sales.csv", file, 0o600); err != nil {
		return err
	}
	fmt.Println("Saved: monthly-sales.csv")
	fmt.Print(string(file))
	unwanted, err := client.Operations.CreateReport(api.CreateReportInput{Body: api.CreateReportBody{Month: "2026-10"}}).Do(ctx)
	if err != nil {
		return err
	}
	cancelled, err := client.Operations.CancelReport(api.CancelReportInput{Path: api.CancelReportPath{ReportID: unwanted.ID}}).Do(ctx)
	if err != nil {
		return err
	}
	if cancelled.Status != "cancelled" {
		return fmt.Errorf("the second report was not cancelled")
	}
	fmt.Println("Second report:", cancelled.Status)
	return nil
}

func main() {
	interrupt, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(interrupt, 10*time.Second)
	defer cancel()
	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
