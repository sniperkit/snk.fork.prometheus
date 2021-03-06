/*
Sniperkit-Bot
- Date: 2018-08-11 23:49:10.542900072 +0200 CEST m=+0.207767820
- Status: analyzed
*/

// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v2"

	"github.com/sniperkit/snk.fork.prometheus/config"
	"github.com/sniperkit/snk.fork.prometheus/pkg/rulefmt"
	"github.com/sniperkit/snk.fork.prometheus/promql"
	"github.com/sniperkit/snk.fork.prometheus/util/promlint"
)

func main() {
	app := kingpin.New(filepath.Base(os.Args[0]), "Tooling for the Prometheus monitoring system.")
	app.Version(version.Print("promtool"))
	app.HelpFlag.Short('h')

	checkCmd := app.Command("check", "Check the resources for validity.")

	checkConfigCmd := checkCmd.Command("config", "Check if the config files are valid or not.")
	configFiles := checkConfigCmd.Arg(
		"config-files",
		"The config files to check.",
	).Required().ExistingFiles()

	checkRulesCmd := checkCmd.Command("rules", "Check if the rule files are valid or not.")
	ruleFiles := checkRulesCmd.Arg(
		"rule-files",
		"The rule files to check.",
	).Required().ExistingFiles()

	checkMetricsCmd := checkCmd.Command("metrics", checkMetricsUsage)

	updateCmd := app.Command("update", "Update the resources to newer formats.")
	updateRulesCmd := updateCmd.Command("rules", "Update rules from the 1.x to 2.x format.")
	ruleFilesUp := updateRulesCmd.Arg("rule-files", "The rule files to update.").Required().ExistingFiles()

	queryCmd := app.Command("query", "Run query against a Prometheus server.")
	queryInstantCmd := queryCmd.Command("instant", "Run instant query.")
	queryServer := queryInstantCmd.Arg("server", "Prometheus server to query.").Required().String()
	queryExpr := queryInstantCmd.Arg("expr", "PromQL query expression.").Required().String()

	queryRangeCmd := queryCmd.Command("range", "Run range query.")
	queryRangeServer := queryRangeCmd.Arg("server", "Prometheus server to query.").Required().String()
	queryRangeExpr := queryRangeCmd.Arg("expr", "PromQL query expression.").Required().String()
	queryRangeBegin := queryRangeCmd.Flag("start", "Query range start time (RFC3339 or Unix timestamp).").String()
	queryRangeEnd := queryRangeCmd.Flag("end", "Query range end time (RFC3339 or Unix timestamp).").String()
	queryRangeStep := queryRangeCmd.Flag("step", "Query step size (duration).").Duration()

	querySeriesCmd := queryCmd.Command("series", "Run series query.")
	querySeriesServer := querySeriesCmd.Arg("server", "Prometheus server to query.").Required().URL()
	querySeriesMatch := querySeriesCmd.Flag("match", "Series selector. Can be specified multiple times.").Required().Strings()
	querySeriesBegin := querySeriesCmd.Flag("start", "Start time (RFC3339 or Unix timestamp).").String()
	querySeriesEnd := querySeriesCmd.Flag("end", "End time (RFC3339 or Unix timestamp).").String()

	debugCmd := app.Command("debug", "Fetch debug information.")
	debugPprofCmd := debugCmd.Command("pprof", "Fetch profiling debug information.")
	debugPprofServer := debugPprofCmd.Arg("server", "Prometheus server to get pprof files from.").Required().String()
	debugMetricsCmd := debugCmd.Command("metrics", "Fetch metrics debug information.")
	debugMetricsServer := debugMetricsCmd.Arg("server", "Prometheus server to get metrics from.").Required().String()
	debugAllCmd := debugCmd.Command("all", "Fetch all debug information.")
	debugAllServer := debugAllCmd.Arg("server", "Prometheus server to get all debug information from.").Required().String()

	queryLabelsCmd := queryCmd.Command("labels", "Run labels query.")
	queryLabelsServer := queryLabelsCmd.Arg("server", "Prometheus server to query.").Required().URL()
	queryLabelsName := queryLabelsCmd.Arg("name", "Label name to provide label values for.").Required().String()

	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case checkConfigCmd.FullCommand():
		os.Exit(CheckConfig(*configFiles...))

	case checkRulesCmd.FullCommand():
		os.Exit(CheckRules(*ruleFiles...))

	case checkMetricsCmd.FullCommand():
		os.Exit(CheckMetrics())

	case updateRulesCmd.FullCommand():
		os.Exit(UpdateRules(*ruleFilesUp...))

	case queryInstantCmd.FullCommand():
		os.Exit(QueryInstant(*queryServer, *queryExpr))

	case queryRangeCmd.FullCommand():
		os.Exit(QueryRange(*queryRangeServer, *queryRangeExpr, *queryRangeBegin, *queryRangeEnd, *queryRangeStep))

	case querySeriesCmd.FullCommand():
		os.Exit(QuerySeries(*querySeriesServer, *querySeriesMatch, *querySeriesBegin, *querySeriesEnd))

	case debugPprofCmd.FullCommand():
		os.Exit(debugPprof(*debugPprofServer))

	case debugMetricsCmd.FullCommand():
		os.Exit(debugMetrics(*debugMetricsServer))

	case debugAllCmd.FullCommand():
		os.Exit(debugAll(*debugAllServer))

	case queryLabelsCmd.FullCommand():
		os.Exit(QueryLabels(*queryLabelsServer, *queryLabelsName))
	}

}

// CheckConfig validates configuration files.
func CheckConfig(files ...string) int {
	failed := false

	for _, f := range files {
		ruleFiles, err := checkConfig(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, "  FAILED:", err)
			failed = true
		} else {
			fmt.Printf("  SUCCESS: %d rule files found\n", len(ruleFiles))
		}
		fmt.Println()

		for _, rf := range ruleFiles {
			if n, err := checkRules(rf); err != nil {
				fmt.Fprintln(os.Stderr, "  FAILED:", err)
				failed = true
			} else {
				fmt.Printf("  SUCCESS: %d rules found\n", n)
			}
			fmt.Println()
		}
	}
	if failed {
		return 1
	}
	return 0
}

func checkFileExists(fn string) error {
	// Nothing set, nothing to error on.
	if fn == "" {
		return nil
	}
	_, err := os.Stat(fn)
	return err
}

func checkConfig(filename string) ([]string, error) {
	fmt.Println("Checking", filename)

	cfg, err := config.LoadFile(filename)
	if err != nil {
		return nil, err
	}

	var ruleFiles []string
	for _, rf := range cfg.RuleFiles {
		rfs, err := filepath.Glob(rf)
		if err != nil {
			return nil, err
		}
		// If an explicit file was given, error if it is not accessible.
		if !strings.Contains(rf, "*") {
			if len(rfs) == 0 {
				return nil, fmt.Errorf("%q does not point to an existing file", rf)
			}
			if err := checkFileExists(rfs[0]); err != nil {
				return nil, fmt.Errorf("error checking rule file %q: %s", rfs[0], err)
			}
		}
		ruleFiles = append(ruleFiles, rfs...)
	}

	for _, scfg := range cfg.ScrapeConfigs {
		if err := checkFileExists(scfg.HTTPClientConfig.BearerTokenFile); err != nil {
			return nil, fmt.Errorf("error checking bearer token file %q: %s", scfg.HTTPClientConfig.BearerTokenFile, err)
		}

		if err := checkTLSConfig(scfg.HTTPClientConfig.TLSConfig); err != nil {
			return nil, err
		}

		for _, kd := range scfg.ServiceDiscoveryConfig.KubernetesSDConfigs {
			if err := checkTLSConfig(kd.TLSConfig); err != nil {
				return nil, err
			}
		}

		for _, filesd := range scfg.ServiceDiscoveryConfig.FileSDConfigs {
			for _, file := range filesd.Files {
				files, err := filepath.Glob(file)
				if err != nil {
					return nil, err
				}
				if len(files) != 0 {
					// There was at least one match for the glob and we can assume checkFileExists
					// for all matches would pass, we can continue the loop.
					continue
				}
				fmt.Printf("  WARNING: file %q for file_sd in scrape job %q does not exist\n", file, scfg.JobName)
			}
		}
	}

	return ruleFiles, nil
}

func checkTLSConfig(tlsConfig config_util.TLSConfig) error {
	if err := checkFileExists(tlsConfig.CertFile); err != nil {
		return fmt.Errorf("error checking client cert file %q: %s", tlsConfig.CertFile, err)
	}
	if err := checkFileExists(tlsConfig.KeyFile); err != nil {
		return fmt.Errorf("error checking client key file %q: %s", tlsConfig.KeyFile, err)
	}

	if len(tlsConfig.CertFile) > 0 && len(tlsConfig.KeyFile) == 0 {
		return fmt.Errorf("client cert file %q specified without client key file", tlsConfig.CertFile)
	}
	if len(tlsConfig.KeyFile) > 0 && len(tlsConfig.CertFile) == 0 {
		return fmt.Errorf("client key file %q specified without client cert file", tlsConfig.KeyFile)
	}

	return nil
}

// CheckRules validates rule files.
func CheckRules(files ...string) int {
	failed := false

	for _, f := range files {
		if n, errs := checkRules(f); errs != nil {
			fmt.Fprintln(os.Stderr, "  FAILED:")
			for _, e := range errs {
				fmt.Fprintln(os.Stderr, e.Error())
			}
			failed = true
		} else {
			fmt.Printf("  SUCCESS: %d rules found\n", n)
		}
		fmt.Println()
	}
	if failed {
		return 1
	}
	return 0
}

func checkRules(filename string) (int, []error) {
	fmt.Println("Checking", filename)

	rgs, errs := rulefmt.ParseFile(filename)
	if errs != nil {
		return 0, errs
	}

	numRules := 0
	for _, rg := range rgs.Groups {
		numRules += len(rg.Rules)
	}

	return numRules, nil
}

// UpdateRules updates the rule files.
func UpdateRules(files ...string) int {
	failed := false

	for _, f := range files {
		if err := updateRules(f); err != nil {
			fmt.Fprintln(os.Stderr, "  FAILED:", err)
			failed = true
		}
	}

	if failed {
		return 1
	}
	return 0
}

func updateRules(filename string) error {
	fmt.Println("Updating", filename)

	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	rules, err := promql.ParseStmts(string(content))
	if err != nil {
		return err
	}

	yamlRG := &rulefmt.RuleGroups{
		Groups: []rulefmt.RuleGroup{{
			Name: filename,
		}},
	}

	yamlRules := make([]rulefmt.Rule, 0, len(rules))

	for _, rule := range rules {
		switch r := rule.(type) {
		case *promql.AlertStmt:
			yamlRules = append(yamlRules, rulefmt.Rule{
				Alert:       r.Name,
				Expr:        r.Expr.String(),
				For:         model.Duration(r.Duration),
				Labels:      r.Labels.Map(),
				Annotations: r.Annotations.Map(),
			})
		case *promql.RecordStmt:
			yamlRules = append(yamlRules, rulefmt.Rule{
				Record: r.Name,
				Expr:   r.Expr.String(),
				Labels: r.Labels.Map(),
			})
		default:
			panic("unknown statement type")
		}
	}

	yamlRG.Groups[0].Rules = yamlRules
	y, err := yaml.Marshal(yamlRG)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename+".yml", y, 0666)
}

var checkMetricsUsage = strings.TrimSpace(`
Pass Prometheus metrics over stdin to lint them for consistency and correctness.

examples:

$ cat metrics.prom | promtool check metrics

$ curl -s http://localhost:9090/metrics | promtool check metrics
`)

// CheckMetrics performs a linting pass on input metrics.
func CheckMetrics() int {
	l := promlint.New(os.Stdin)
	problems, err := l.Lint()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error while linting:", err)
		return 1
	}

	for _, p := range problems {
		fmt.Fprintln(os.Stderr, p.Metric, p.Text)
	}

	if len(problems) > 0 {
		return 3
	}

	return 0
}

// QueryInstant performs an instant query against a Prometheus server.
func QueryInstant(url string, query string) int {
	config := api.Config{
		Address: url,
	}

	// Create new client.
	c, err := api.NewClient(config)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error creating API client:", err)
		return 1
	}

	// Run query against client.
	api := v1.NewAPI(c)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	val, err := api.Query(ctx, query, time.Now())
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "query error:", err)
		return 1
	}

	fmt.Println(val.String())

	return 0
}

// QueryRange performs a range query against a Prometheus server.
func QueryRange(url, query, start, end string, step time.Duration) int {
	config := api.Config{
		Address: url,
	}

	// Create new client.
	c, err := api.NewClient(config)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error creating API client:", err)
		return 1
	}

	var stime, etime time.Time

	if end == "" {
		etime = time.Now()
	} else {
		etime, err = parseTime(end)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error parsing end time:", err)
			return 1
		}
	}

	if start == "" {
		stime = etime.Add(-5 * time.Minute)
	} else {
		stime, err = parseTime(start)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error parsing start time:", err)
		}
	}

	if !stime.Before(etime) {
		fmt.Fprintln(os.Stderr, "start time is not before end time")
	}

	if step == 0 {
		resolution := math.Max(math.Floor(etime.Sub(stime).Seconds()/250), 1)
		// Convert seconds to nanoseconds such that time.Duration parses correctly.
		step = time.Duration(resolution) * time.Second
	}

	// Run query against client.
	api := v1.NewAPI(c)
	r := v1.Range{Start: stime, End: etime, Step: step}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	val, err := api.QueryRange(ctx, query, r)
	cancel()

	if err != nil {
		fmt.Fprintln(os.Stderr, "query error:", err)
		return 1
	}

	fmt.Println(val.String())
	return 0
}

// QuerySeries queries for a series against a Prometheus server.
func QuerySeries(url *url.URL, matchers []string, start string, end string) int {
	config := api.Config{
		Address: url.String(),
	}

	// Create new client.
	c, err := api.NewClient(config)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error creating API client:", err)
		return 1
	}

	// TODO: clean up timestamps
	var (
		minTime = time.Now().Add(-9999 * time.Hour)
		maxTime = time.Now().Add(9999 * time.Hour)
	)

	var stime, etime time.Time

	if start == "" {
		stime = minTime
	} else {
		stime, err = parseTime(start)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error parsing start time:", err)
		}
	}

	if end == "" {
		etime = maxTime
	} else {
		etime, err = parseTime(end)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error parsing end time:", err)
		}
	}

	// Run query against client.
	api := v1.NewAPI(c)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	val, err := api.Series(ctx, matchers, stime, etime)
	cancel()

	if err != nil {
		fmt.Fprintln(os.Stderr, "query error:", err)
		return 1
	}

	for _, v := range val {
		fmt.Println(v)
	}
	return 0
}

// QueryLabels queries for label values against a Prometheus server.
func QueryLabels(url *url.URL, name string) int {
	config := api.Config{
		Address: url.String(),
	}

	// Create new client.
	c, err := api.NewClient(config)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error creating API client:", err)
		return 1
	}

	// Run query against client.
	api := v1.NewAPI(c)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	val, err := api.LabelValues(ctx, name)
	cancel()

	if err != nil {
		fmt.Fprintln(os.Stderr, "query error:", err)
		return 1
	}

	for _, v := range val {
		fmt.Println(v)
	}
	return 0
}

func parseTime(s string) (time.Time, error) {
	if t, err := strconv.ParseFloat(s, 64); err == nil {
		s, ns := math.Modf(t)
		return time.Unix(int64(s), int64(ns*float64(time.Second))), nil
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("cannot parse %q to a valid timestamp", s)
}

func debugPprof(url string) int {
	w, err := newDebugWriter(debugWriterConfig{
		serverURL:   url,
		tarballName: "debug.tar.gz",
		pathToFileName: map[string]string{
			"/debug/pprof/block":        "block.pb",
			"/debug/pprof/goroutine":    "goroutine.pb",
			"/debug/pprof/heap":         "heap.pb",
			"/debug/pprof/mutex":        "mutex.pb",
			"/debug/pprof/threadcreate": "threadcreate.pb",
		},
		postProcess: pprofPostProcess,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error creating debug writer:", err)
		return 1
	}
	return w.Write()
}

func debugMetrics(url string) int {
	w, err := newDebugWriter(debugWriterConfig{
		serverURL:   url,
		tarballName: "debug.tar.gz",
		pathToFileName: map[string]string{
			"/metrics": "metrics.txt",
		},
		postProcess: metricsPostProcess,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error creating debug writer:", err)
		return 1
	}
	return w.Write()
}

func debugAll(url string) int {
	w, err := newDebugWriter(debugWriterConfig{
		serverURL:   url,
		tarballName: "debug.tar.gz",
		pathToFileName: map[string]string{
			"/debug/pprof/block":        "block.pb",
			"/debug/pprof/goroutine":    "goroutine.pb",
			"/debug/pprof/heap":         "heap.pb",
			"/debug/pprof/mutex":        "mutex.pb",
			"/debug/pprof/threadcreate": "threadcreate.pb",
			"/metrics":                  "metrics.txt",
		},
		postProcess: allPostProcess,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error creating debug writer:", err)
		return 1
	}
	return w.Write()
}
