package main

type status map[string]ProdStatus

func makeStatus() status {
	return make(map[string]ProdStatus)
}

// Append and keep latest N for each test (TODO)
func (s status) appendTestResult(host, product, group string, tr *TestResult) {
	if _, ok := s[host]; !ok {
		s[host] = makeProdStatus()
	}
	if _, ok := s[host][product]; !ok {
		s[host][product] = makeGroupStatus()
	}
	if _, ok := s[host][product][group]; !ok {
		s[host][product][group] = make([]*TestResult, 1)
	}
	s[host][product][group][0] = tr
}

type ProdStatus map[string]GroupStatus

func makeProdStatus() ProdStatus {
	return make(map[string]GroupStatus)
}

type GroupStatus map[string][]*TestResult

func makeGroupStatus() GroupStatus {
	return make(map[string][]*TestResult)
}
