package domain

import "fmt"

func GetDataBaseDSN(path string) string {
	return fmt.Sprintf(
		"file:%s?"+
			"_pragma=journal_mode(WAL)&"+
			"_pragma=busy_timeout(5000)&"+
			"_pragma=foreign_keys(1)&"+
			"_pragma=synchronous(NORMAL)&"+
			"_pragma=temp_store(MEMORY)",
		path,
	)
}
