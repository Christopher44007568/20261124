package sim

import "fmt"

type Policy string

const (
	PolicyEqual           Policy = "equal"
	PolicyGazeRegion      Policy = "gaze-region"
	PolicyHistoricalError Policy = "history-error"
)

var Policies = [...]Policy{PolicyEqual, PolicyGazeRegion, PolicyHistoricalError}

func ParsePolicy(value string) (Policy, error) {
	for _, policy := range Policies {
		if string(policy) == value {
			return policy, nil
		}
	}
	return "", fmt.Errorf("unknown policy %q", value)
}

func (p Policy) Label() string {
	switch p {
	case PolicyEqual:
		return "equal"
	case PolicyGazeRegion:
		return "gaze-region"
	case PolicyHistoricalError:
		return "history-error"
	default:
		return string(p)
	}
}

func (p Policy) Description() string {
	switch p {
	case PolicyEqual:
		return "四任务轮转升级，预算削减由双眼和中心/外围共同承担"
	case PolicyGazeRegion:
		return "先升级双眼中心，再升级外围；单眼额度不互借"
	case PolicyHistoricalError:
		return "每轮升级历史误差最大的任务，平手时中心优先"
	default:
		return ""
	}
}
