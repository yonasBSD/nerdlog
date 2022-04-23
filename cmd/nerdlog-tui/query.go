package main

// QueryFull contains everything that defines a query: the hosts filter, time range,
// and the query to filter logs.
type QueryFull struct {
	HostsFilter string
	Time        string
	Query       string
}
