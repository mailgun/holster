package sandra

import (
	"fmt"

	"github.com/gocql/gocql"
)

type TestErrorCassandra struct{}

func (c *TestErrorCassandra) Query(consistency gocql.Consistency, queryString string, queryParams ...interface{}) *gocql.Query {
	return nil
}

func (c *TestErrorCassandra) ExecuteQuery(queryString string, queryParams ...interface{}) error {
	return fmt.Errorf("Error during ExecuteQuery")
}

func (c *TestErrorCassandra) ExecuteBatch(batchType gocql.BatchType, queries []string, params [][]interface{}) error {
	return fmt.Errorf("Error during ExecuteBatch")
}

func (c *TestErrorCassandra) ExecuteUnloggedBatch(queries []string, params [][]interface{}) error {
	return fmt.Errorf("Error during ExecuteUnloggedBatch")
}

func (c *TestErrorCassandra) ScanQuery(queryString string, queryParams []interface{}, outParams ...interface{}) error {
	return fmt.Errorf("Error during ScanQuery")
}

func (c *TestErrorCassandra) ScanCASQuery(queryString string, queryParams []interface{}, outParams ...interface{}) (bool, error) {
	return false, fmt.Errorf("Error during ScanCASQuery")
}

func (c *TestErrorCassandra) IterQuery(queryString string, queryParams []interface{}, outParams ...interface{}) func() (int, bool, error) {
	return func() (int, bool, error) {
		return 0, true, fmt.Errorf("Error during IterQuery")
	}
}

func (c *TestErrorCassandra) Close() error {
	return fmt.Errorf("Error during Close")
}
