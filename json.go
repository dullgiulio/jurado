package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type jsonTester struct {
	resp  *http.Response
	body  []byte
	obj   interface{}
	tests map[string]testFn
}

// TODO: Check structure of Options.
func (j *jsonTester) init(ch *Check) error {
	if _, ok := ch.Options["Url"]; !ok {
		return fmt.Errorf("need Url field in check options")
	}
	j.tests = testMap(make(map[string]testFn))
	for k := range ch.Tests {
		var fn testFn
		args := ch.Tests[k].Arguments
		switch k {
		case "json-object-path":
			var path string
			val, ok := args["path"]
			if ok {
				if path, ok = val.(string); !ok {
					return fmt.Errorf("%s: need to speficy a string as 'path' value", k)
				}
			}
			keys, err := parseJsonPath(path)
			if err != nil {
				return fmt.Errorf("%s: invalid path: %s", k, err)
			}
			fn = j.testJsonPath(keys)
		default:
			return fmt.Errorf("unknown test %s", k)
		}
		j.tests[k] = fn
	}
	return nil
}

func (j *jsonTester) setUp(ch *Check) error {
	var err error
	j.body, j.resp, err = httpRequest(ch.Options)
	if err != nil {
		return fmt.Errorf("cannot request JSON content: %s", err)
	}
	if err = json.Unmarshal(j.body, &j.obj); err != nil {
		return fmt.Errorf("cannot unmarshal JSON: %s", err)
	}
	return nil
}

func (j *jsonTester) tearDown() error {
	return nil
}

func (j *jsonTester) get(name string) testFn {
	return j.tests[name]
}

func (j *jsonTester) testJsonPath(path *jsonKeys) testFn {
	return func(ch *Check) *TestResult {
		var ok bool
		obj := j.obj
		for i := range path.keys {
			obj, ok = path.keys[i].value(obj)
			if !ok {
				return NewTestResultError(500, fmt.Sprintf("JSON object only contains path '%s', not '...%s'", path.join(0, i), path.join(i, -1)))
			}
		}
		return NewTestResult(200)
	}
}

type jsonKeys struct {
	skeys []string
	keys  []jsonKey
}

func parseJsonPath(path string) (*jsonKeys, error) {
	parts := strings.Split(path, ".")
	keys := make([]jsonKey, len(parts))
	for i := range parts {
		if parts[i] == "" {
			return nil, fmt.Errorf("path '%s' contains empty part after %d parts", path, i)
		}
		p := parts[i]
		v, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			keys[i] = stringKey(p)
			continue
		}
		keys[i] = arrayKey(int(v))
	}
	return &jsonKeys{parts, keys}, nil
}

func (k *jsonKeys) join(i, j int) string {
	if j <= 0 {
		j = len(k.skeys)
	}
	return strings.Join(k.skeys[i:j], ".")
}

type jsonKey interface {
	value(obj interface{}) (interface{}, bool)
}

type stringKey string

func (s stringKey) value(obj interface{}) (interface{}, bool) {
	mobj, ok := obj.(map[string]interface{})
	if !ok {
		return nil, false
	}
	if obj, ok = mobj[string(s)]; ok {
		return obj, true
	}
	fmt.Printf("no map %s\n", string(s))
	return nil, false
}

type arrayKey int

func (i arrayKey) value(obj interface{}) (interface{}, bool) {
	aobj, ok := obj.([]interface{})
	if !ok {
		fmt.Printf("not an array %#v\n", obj)
		return nil, false
	}
	if len(aobj) < int(i) {
		fmt.Printf("%d too big for %d\n", int(i), len(aobj))
		return nil, false
	}
	return aobj[int(i)], true
}
