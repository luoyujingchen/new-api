package setting

import (
	"fmt"
	"strconv"
	"strings"
)

var QueueEnabled = true
var QueueDefaultTimeout = 300
var QueueMaxTimeout = 3600
var QueueGlobalMaxSize = 0

func NormalizeQueuePriority(priority int) int {
	switch {
	case priority == 0:
		return 5
	case priority < 1:
		return 1
	case priority > 10:
		return 10
	default:
		return priority
	}
}

func NormalizeQueueTimeoutOption(timeout int) int {
	if timeout < 0 {
		return 0
	}
	return timeout
}

func parseNonNegativeQueueValue(raw string, field string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", field)
	}
	if value < 0 {
		return 0, fmt.Errorf("%s must be greater than or equal to 0", field)
	}
	return value, nil
}

func CheckQueueDefaultTimeout(raw string) error {
	value, err := parseNonNegativeQueueValue(raw, "QueueDefaultTimeout")
	if err != nil {
		return err
	}
	if value == 0 {
		return fmt.Errorf("QueueDefaultTimeout must be greater than 0")
	}
	return nil
}

func CheckQueueGlobalMaxSize(raw string) error {
	_, err := parseNonNegativeQueueValue(raw, "QueueGlobalMaxSize")
	return err
}

func CheckQueueMaxTimeout(raw string) error {
	value, err := parseNonNegativeQueueValue(raw, "QueueMaxTimeout")
	if err != nil {
		return err
	}
	if value == 0 {
		return fmt.Errorf("QueueMaxTimeout must be greater than 0")
	}
	return nil
}