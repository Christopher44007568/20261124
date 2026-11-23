package sim

const historyAlpha = 0.35

type framePlan struct {
	alloc   allocation
	floorOK bool
	flags   FallbackFlags
}

type Simulator struct {
	profile              Profile
	history              [2][2]float64
	predictionFailStreak int
}

func NewSimulator() *Simulator {
	return NewSimulatorWithProfile(DefaultProfile())
}

func NewSimulatorWithProfile(profile Profile) *Simulator {
	return &Simulator{profile: profile}
}

func (s *Simulator) RunScenario(scenario Scenario, policy Policy) ScenarioReport {
	s.resetHistory()
	s.predictionFailStreak = 0

	results := make([]FrameResult, 0, len(scenario.Frames))
	for _, frame := range scenario.Frames {
		if frame.PredictionOK {
			s.predictionFailStreak = 0
		} else {
			s.predictionFailStreak++
		}
		hold := s.predictionFailStreak >= 3
		plan := s.planFrame(frame, policy, hold)
		result := s.frameResult(frame, plan)
		results = append(results, result)
		s.observe(result)
	}

	return ScenarioReport{
		Scenario: scenario,
		Metrics:  aggregate(scenario, policy, results),
		Frames:   results,
	}
}

func (s *Simulator) resetHistory() {
	for _, eye := range Eyes {
		for _, region := range Regions {
			s.history[eye][region] = s.profile.Spec(region, TierMin).Error
		}
	}
}

func (s *Simulator) planFrame(frame Frame, policy Policy, predictionHold bool) framePlan {
	plan := framePlan{}
	if !plan.alloc.reserveFloor(s.profile) {
		plan.flags.Floor = true
		if predictionHold {
			plan.flags.Prediction = true
		}
		return plan
	}

	ok, flags := plan.alloc.fits(s.profile, frame.Budget)
	if !ok {
		plan.flags = plan.flags.Merge(flags)
		plan.flags.Floor = true
		if predictionHold {
			plan.flags.Prediction = true
		}
		return plan
	}
	plan.floorOK = true

	if predictionHold {
		plan.flags.Prediction = true
		return plan
	}

	switch policy {
	case PolicyEqual:
		plan.flags = plan.flags.Merge(allocateEqual(&plan.alloc, s.profile, frame.Budget))
	case PolicyGazeRegion:
		plan.flags = plan.flags.Merge(allocateGazeRegion(&plan.alloc, s.profile, frame.Budget))
	case PolicyHistoricalError:
		plan.flags = plan.flags.Merge(allocateHistoricalError(
			&plan.alloc,
			s.profile,
			frame.Budget,
			s.history,
		))
	}
	return plan
}

func (s *Simulator) frameResult(frame Frame, plan framePlan) FrameResult {
	result := FrameResult{
		Index:          frame.Index,
		PredictionOK:   frame.PredictionOK,
		PredictionHold: plan.flags.Prediction,
		FloorPreserved: plan.floorOK,
		OfferedTasks:   4,
		Fallbacks:      plan.flags,
	}
	if !plan.floorOK {
		result.CenterError = fallbackError(Center)
		result.PeripheralError = fallbackError(Periphery)
		return result
	}

	for _, key := range JobKeys() {
		tier := plan.alloc.tiers[key.Eye][key.Region]
		spec := s.profile.Spec(key.Region, tier)
		result.Submitted = append(result.Submitted, Submission{
			Key:  key,
			Tier: tier,
			Spec: spec,
		})
		result.SamplesUsed += spec.Samples
		result.RedrawUsed += spec.Redraw
		result.EyeBufferUsed[key.Eye] += spec.EyeBuffer
	}
	result.CenterError = submittedRegionError(s.profile, result.Submitted, Center)
	result.PeripheralError = submittedRegionError(s.profile, result.Submitted, Periphery)
	return result
}

func submittedRegionError(profile Profile, submissions []Submission, region Region) float64 {
	byKey := make(map[JobKey]Submission, len(submissions))
	for _, submission := range submissions {
		byKey[submission.Key] = submission
	}

	total := 0.0
	for _, eye := range Eyes {
		if submission, ok := byKey[JobKey{Eye: eye, Region: region}]; ok {
			total += submission.Spec.Error
		} else {
			total += fallbackError(region)
		}
	}
	return total / float64(len(Eyes))
}

func (s *Simulator) observe(result FrameResult) {
	byKey := make(map[JobKey]Submission, len(result.Submitted))
	for _, submission := range result.Submitted {
		byKey[submission.Key] = submission
	}
	for _, key := range JobKeys() {
		observed := fallbackError(key.Region)
		if submission, ok := byKey[key]; ok {
			observed = submission.Spec.Error
		}
		s.history[key.Eye][key.Region] = historyAlpha*observed +
			(1-historyAlpha)*s.history[key.Eye][key.Region]
	}
}

func aggregate(scenario Scenario, policy Policy, frames []FrameResult) Metrics {
	metrics := Metrics{
		Scenario: scenario.Name,
		Policy:   policy,
		Frames:   len(frames),
	}

	used := resourceUse{}
	var budgetSamples, budgetRedraw, budgetEye int
	for i, frame := range frames {
		metrics.CenterError += frame.CenterError
		metrics.PeripheralError += frame.PeripheralError
		metrics.OfferedTasks += frame.OfferedTasks
		metrics.SubmittedTasks += frame.SubmittedTasks()
		if frame.FloorPreserved {
			metrics.FloorPreservedFrames++
		} else {
			metrics.FloorFailureFrames++
		}

		used.samples += frame.SamplesUsed
		used.redraw += frame.RedrawUsed
		used.eye[Left] += frame.EyeBufferUsed[Left]
		used.eye[Right] += frame.EyeBufferUsed[Right]

		budget := scenario.Frames[i].Budget
		budgetSamples += budget.Samples
		budgetRedraw += budget.Redraw
		budgetEye += budget.EyeBufferTotal()

		if frame.Fallbacks.Budget {
			metrics.Fallbacks.Budget++
		}
		if frame.Fallbacks.EyeCap {
			metrics.Fallbacks.EyeCap++
		}
		if frame.Fallbacks.Prediction {
			metrics.Fallbacks.Prediction++
		}
		if frame.Fallbacks.Floor {
			metrics.Fallbacks.Floor++
		}
	}

	denominator := float64(len(frames))
	if denominator > 0 {
		metrics.CenterError /= denominator
		metrics.PeripheralError /= denominator
		metrics.CompletionRate = float64(metrics.SubmittedTasks) / float64(metrics.OfferedTasks)
	}
	metrics.SampleUse = ratio(used.samples, budgetSamples)
	metrics.RedrawUse = ratio(used.redraw, budgetRedraw)
	metrics.EyeBufferUse = ratio(used.eye[Left]+used.eye[Right], budgetEye)
	return metrics
}

func ratio(used, budget int) float64 {
	if budget <= 0 {
		return 0
	}
	return float64(used) / float64(budget)
}

func RunScenario(scenario Scenario, policy Policy) ScenarioReport {
	return NewSimulator().RunScenario(scenario, policy)
}

func RunAll(scenarios []Scenario, policies []Policy) []ScenarioReport {
	reports := make([]ScenarioReport, 0, len(scenarios)*len(policies))
	for _, scenario := range scenarios {
		for _, policy := range policies {
			reports = append(reports, RunScenario(scenario, policy))
		}
	}
	return reports
}
