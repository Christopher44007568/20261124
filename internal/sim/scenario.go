package sim

import "fmt"

type Scenario struct {
	Name        string  `json:"name"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Frames      []Frame `json:"frames"`
}

func normalBudget() Budget {
	return Budget{
		Samples:   36,
		Redraw:    8,
		EyeBuffer: [2]int{8, 8},
	}
}

func collapsedBudget() Budget {
	return Budget{
		Samples:   22,
		Redraw:    5,
		EyeBuffer: [2]int{5, 5},
	}
}

func frames(count int, budget func() Budget, predictionOK func(int) bool) []Frame {
	out := make([]Frame, 0, count)
	for i := 0; i < count; i++ {
		ok := true
		if predictionOK != nil {
			ok = predictionOK(i)
		}
		out = append(out, Frame{Index: i, Budget: budget(), PredictionOK: ok})
	}
	return out
}

func buildScenario(name, title, description string, frameList []Frame) Scenario {
	return Scenario{Name: name, Title: title, Description: description, Frames: frameList}
}

func BaselineScenario() Scenario {
	return buildScenario(
		"baseline",
		"预算充足",
		"全部资源足以提交 full 档，用于验证策略本身不应制造回退。",
		frames(20, normalBudget, nil),
	)
}

func BudgetDropScenario() Scenario {
	all := make([]Frame, 0, 30)
	all = append(all, frames(6, normalBudget, nil)...)
	for i := 6; i < 18; i++ {
		all = append(all, Frame{Index: i, Budget: collapsedBudget(), PredictionOK: true})
	}
	all = append(all, frames(12, normalBudget, nil)...)
	for i := range all {
		all[i].Index = i
	}
	return buildScenario(
		"budget-drop",
		"预算骤降",
		"索引 6–17 采样点由 36 降至 22、补绘由 8 降至 5、单眼缓冲由 8 降至 5，索引 18 恢复。",
		all,
	)
}

func PredictionFailureScenario() Scenario {
	return buildScenario(
		"prediction-fail",
		"连续预测失败",
		"索引 6–11 连续预测失败；连续第 3 个失败帧开始进入保守回退。",
		frames(20, normalBudget, func(i int) bool {
			return i < 6 || i >= 12
		}),
	)
}

func SingleEyeOverrunScenario() Scenario {
	budget := func() Budget {
		return Budget{
			Samples:   32,
			Redraw:    6,
			EyeBuffer: [2]int{4, 7},
		}
	}
	return buildScenario(
		"single-eye-overrun",
		"单眼缓冲超额",
		"左眼 full 档需要 7 单位但硬上限只有 4；右眼可到 7，禁止借用左眼额度。",
		frames(20, budget, nil),
	)
}

func CombinedScenario() Scenario {
	all := make([]Frame, 0, 32)
	all = append(all, frames(6, normalBudget, nil)...)
	for i := 6; i < 16; i++ {
		b := Budget{Samples: 22, Redraw: 5, EyeBuffer: [2]int{4, 5}}
		all = append(all, Frame{
			Index:        i,
			Budget:       b,
			PredictionOK: i < 10,
		})
	}
	for i := 16; i < 24; i++ {
		b := Budget{Samples: 30, Redraw: 6, EyeBuffer: [2]int{4, 7}}
		all = append(all, Frame{Index: i, Budget: b, PredictionOK: true})
	}
	all = append(all, frames(8, normalBudget, nil)...)
	for i := range all {
		all[i].Index = i
	}
	return buildScenario(
		"combined",
		"组合压力",
		"预算骤降、索引 10–15 预测失败、左眼 4 单位上限和恢复期依次叠加。",
		all,
	)
}

func StandardScenarios() []Scenario {
	return []Scenario{
		BaselineScenario(),
		BudgetDropScenario(),
		PredictionFailureScenario(),
		SingleEyeOverrunScenario(),
		CombinedScenario(),
	}
}

func ScenarioByName(name string) (Scenario, error) {
	for _, scenario := range StandardScenarios() {
		if scenario.Name == name {
			return scenario, nil
		}
	}
	return Scenario{}, fmt.Errorf("unknown scenario %q", name)
}
