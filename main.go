package main

import (
	"fmt"

	"xrstatemachine/xr"
)

func main() {
	fmt.Println("XR 渲染状态机实验: NORMAL / LOST / RELOCALIZING / RECOVERY")
	fmt.Println("策略: freeze(冻结) | inertial(限时惯性外推) | fade(淡入淡出)")
	fmt.Println()
	fmt.Print(xr.S1Outage())
	fmt.Println()
	fmt.Print(xr.S2CrossRoom())
	fmt.Println()
	fmt.Print(xr.S3ClockJump())
}
