package main

type status map[string]ProdStatus

func makeStatus() status {
	return make(map[string]ProdStatus)
}

// TODO: Append and keep latest N for each test
func (s status) add(cr *CheckResult) {
	if _, ok := s[cr.Host]; !ok {
		s[cr.Host] = makeProdStatus()
	}
	if _, ok := s[cr.Host][cr.Product]; !ok {
		s[cr.Host][cr.Product] = makeGroupStatus()
	}
	if _, ok := s[cr.Host][cr.Product][cr.Group]; !ok {
		s[cr.Host][cr.Product][cr.Group] = make([]*CheckResult, 1)
	}
	s[cr.Host][cr.Product][cr.Group][0] = cr
}

type ProdStatus map[string]GroupStatus

func makeProdStatus() ProdStatus {
	return make(map[string]GroupStatus)
}

type GroupStatus map[string][]*CheckResult

func makeGroupStatus() GroupStatus {
	return make(map[string][]*CheckResult)
}
