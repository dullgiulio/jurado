package main

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

type mysqlTester struct {
	tests map[string]testFn
	dsn   string
	query string
	count int
}

func (m *mysqlTester) init(ch *Check) error {
	dsn, ok := ch.Options["DSN"]
	if !ok {
		return fmt.Errorf("need DSN field in check options")
	}
	if m.dsn, ok = dsn.(string); !ok {
		return fmt.Errorf("need DSN option to be a string")
	}
	m.tests = testMap(make(map[string]testFn))
	for k := range ch.Tests {
		var fn testFn
		args := ch.Tests[k].Arguments
		switch k {
		case "mysql-query-count-nonzero":
			val, ok := args["query"]
			if !ok {
				return fmt.Errorf("%s: need to specify a 'query' option", k)
			}
			if m.query, ok = val.(string); !ok {
				return fmt.Errorf("%s: need to specify a string as 'query' value", k)
			}
			fn = m.testQueryCount()
		default:
			return fmt.Errorf("unknown test %s", k)
		}
		m.tests[k] = fn
	}
	return nil
}

func (m *mysqlTester) setUp(ch *Check) error {
	db, err := sql.Open("mysql", m.dsn)
	if err != nil {
		return fmt.Errorf("cannot open connection: %s", err)
	}
	defer db.Close()
	row := db.QueryRow(m.query)
	if err = row.Scan(&m.count); err != nil {
		return fmt.Errorf("cannot execute count query: %s", err)
	}
	return nil
}

func (m *mysqlTester) tearDown() error {
	return nil
}

func (m *mysqlTester) get(name string) testFn {
	return m.tests[name]
}

func (m *mysqlTester) testQueryCount() testFn {
	return func(ch *Check) *TestResult {
		if m.count == 0 {
			return NewTestResultError(500, "Query returned zero count")
		}
		return NewTestResult(200)
	}
}
