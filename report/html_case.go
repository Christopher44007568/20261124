package report

import (
	"fmt"
	"strings"
)

func caseHTML(c Case, n int) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<section class="case" id="%s">`, c.ID)
	b.WriteString(`<div class="casehead"><div>`)
	fmt.Fprintf(&b, `<div class="kick">case %02d / %s</div><h2>%s</h2><p class="lead">%s</p></div>`, n, c.ID, c.Title, c.Note)
	b.WriteString(`</div><div class="cards">`)
	fmt.Fprintf(&b, `<div class="card"><span>rotation coverage</span><b>%s</b><small>valid</small></div>`, pct(c.RotStats.Coverage))
	fmt.Fprintf(&b, `<div class="card"><span>depth coverage</span><b>%s</b><small>valid</small></div>`, pct(c.DepthStat.Coverage))
	fmt.Fprintf(&b, `<div class="card"><span>rotation MAE</span><b>%.2f</b><small>0–255</small></div>`, c.RotStats.MAE)
	fmt.Fprintf(&b, `<div class="card"><span>depth MAE</span><b>%.2f</b><small>0–255</small></div>`, c.DepthStat.MAE)
	b.WriteString(`</div><div class="callout"><b>运动</b><p>`)
	b.WriteString(escape(c.Motion))
	b.WriteString(`</p></div>`)

	b.WriteString(`<h3>目标真值 / 纯旋转 / 深度重投影</h3><div class="grid3">`)
	fmt.Fprintf(&b, `<figure><img src="%s_gt.png"><figcaption><b>目标真值</b><span class="tag">ground truth</span></figcaption></figure>`, c.ID)
	fmt.Fprintf(&b, `<figure><img src="%s_rotation.png"><figcaption><b>纯旋转 warp</b><span class="tag">no depth</span></figcaption></figure>`, c.ID)
	fmt.Fprintf(&b, `<figure><img src="%s_depth.png"><figcaption><b>深度平移重投影</b><span class="tag">source depth</span></figcaption></figure>`, c.ID)
	b.WriteString(`</div>`)

	b.WriteString(`<h3>语义区域与拒绝掩码</h3><div class="grid3">`)
	fmt.Fprintf(&b, `<figure><img src="%s_regions.png"><figcaption><b>目标区域</b><span class="tag">room / pole / specular</span></figcaption></figure>`, c.ID)
	fmt.Fprintf(&b, `<figure><img src="%s_rotation_mask.png"><figcaption><b>旋转拒绝掩码</b><span class="tag">%s OOB</span></figcaption></figure>`, c.ID, comma(c.RotStats.OutOfBounds))
	fmt.Fprintf(&b, `<figure><img src="%s_depth_mask.png"><figcaption><b>深度拒绝掩码</b><span class="tag">%s holes</span></figcaption></figure>`, c.ID, comma(c.DepthStat.NoCoverage))
	b.WriteString(`</div>`)

	b.WriteString(`<h3>误差热图</h3><div class="grid2">`)
	fmt.Fprintf(&b, `<figure><img src="%s_rotation_heat.png"><figcaption><b>纯旋转误差</b><span class="tag">MAE %.2f</span></figcaption></figure>`, c.ID, c.RotStats.MAE)
	fmt.Fprintf(&b, `<figure><img src="%s_depth_heat.png"><figcaption><b>深度重投影误差</b><span class="tag">MAE %.2f</span></figcaption></figure>`, c.ID, c.DepthStat.MAE)
	b.WriteString(`</div>`)
	b.WriteString(summaryTable(c))
	b.WriteString(regionTable(c))
	b.WriteString(focusHTML(c))
	b.WriteString(`</section>`)
	return b.String()
}

func summaryTable(c Case) string {
	rows := []struct {
		name string
		r, d string
	}{
		{"覆盖率", pct(c.RotStats.Coverage), pct(c.DepthStat.Coverage)},
		{"有效像素", comma(c.RotStats.Valid), comma(c.DepthStat.Valid)},
		{"拒绝像素", comma(c.RotStats.Rejected), comma(c.DepthStat.Rejected)},
		{"MAE (0–255)", fmt.Sprintf("%.2f", c.RotStats.MAE), fmt.Sprintf("%.2f", c.DepthStat.MAE)},
		{"RMSE (0–255)", fmt.Sprintf("%.2f", c.RotStats.RMSE), fmt.Sprintf("%.2f", c.DepthStat.RMSE)},
		{"坏像素率", pct(c.RotStats.BadPixelRate), pct(c.DepthStat.BadPixelRate)},
		{"越界采样拒绝", comma(c.RotStats.OutOfBounds), comma(c.DepthStat.OutOfBounds)},
		{"无覆盖空洞", comma(c.RotStats.NoCoverage), comma(c.DepthStat.NoCoverage)},
		{"源深度缺失像素", comma(c.RotStats.SourceNoDepth), comma(c.DepthStat.SourceNoDepth)},
		{"源投影越出目标", comma(c.RotStats.SourceOffscreen), comma(c.DepthStat.SourceOffscreen)},
	}
	var b strings.Builder
	b.WriteString(`<h3>整帧指标</h3><table><thead><tr><th>指标</th><th>纯旋转</th><th>深度重投影</th></tr></thead><tbody>`)
	for _, row := range rows {
		fmt.Fprintf(&b, `<tr><td>%s</td><td class="rot mono">%s</td><td class="dep mono">%s</td></tr>`, row.name, row.r, row.d)
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}

func regionTable(c Case) string {
	names := []string{"room", "near pole", "static specular"}
	var b strings.Builder
	b.WriteString(`<h3>分区检查</h3><table><thead><tr><th>区域</th><th>旋转 MAE</th><th>旋转覆盖</th><th>深度 MAE</th><th>深度覆盖</th></tr></thead><tbody>`)
	for _, name := range names {
		fmt.Fprintf(&b, `<tr><td>%s</td><td class="rot mono">%.2f</td><td class="rot mono">%s</td><td class="dep mono">%.2f</td><td class="dep mono">%s</td></tr>`,
			name, regionMAE(c.RotStats, name), pct(regionCoverage(c.RotStats, name)),
			regionMAE(c.DepthStat, name), pct(regionCoverage(c.DepthStat, name)))
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}

func focusHTML(c Case) string {
	var b strings.Builder
	b.WriteString(`<div class="grid2">`)
	fmt.Fprintf(&b, `<figure class="focus"><img src="%s_pole_focus.png"><figcaption><b>近杆放大</b><span>真值</span><span>纯旋转</span><span>深度</span></figcaption></figure>`, c.ID)
	fmt.Fprintf(&b, `<figure class="focus"><img src="%s_specular_focus.png"><figcaption><b>静止高光放大</b><span>真值</span><span>纯旋转</span><span>深度</span></figcaption></figure>`, c.ID)
	b.WriteString(`</div>`)
	return b.String()
}

func invariantHTML(data Data) string {
	fabricated := 0
	rotOOB, depthOOB, sourceOff := 0, 0, 0
	sourceNoDepth := 0
	for i := range data.Cases {
		c := &data.Cases[i]
		for _, ok := range c.Rotation.DepthValid {
			if ok {
				fabricated++
			}
		}
		rotOOB += c.RotStats.OutOfBounds
		depthOOB += c.DepthStat.OutOfBounds
		sourceOff += c.DepthStat.SourceOffscreen
		if c.DepthStat.SourceNoDepth > sourceNoDepth {
			sourceNoDepth = c.DepthStat.SourceNoDepth
		}
	}
	status := func(ok bool) string {
		if ok {
			return `<span style="color:#46d68c">PASS</span>`
		}
		return `<span style="color:#ed4034">FAIL</span>`
	}
	var b strings.Builder
	b.WriteString(`<section class="case"><h2>实测不变量</h2><p>下面不是文字承诺，而是两个运动场景实际运行后从结果缓冲、拒绝掩码和采样统计中读出的数。</p><table><thead><tr><th>检查</th><th>实测</th><th>结果</th></tr></thead><tbody>`)
	fmt.Fprintf(&b, `<tr><td>纯旋转输出中标记为有效的深度像素</td><td class="mono">%s / %s</td><td>%s</td></tr>`,
		comma(fabricated), comma(totalRotationPixels(data)), status(fabricated == 0))
	fmt.Fprintf(&b, `<tr><td>纯旋转越界采样拒绝（两个场景合计）</td><td class="mono">%s</td><td>原始帧外坐标被拒绝</td></tr>`, comma(rotOOB))
	fmt.Fprintf(&b, `<tr><td>深度路径越界采样次数（两个场景合计）</td><td class="mono">%s</td><td>%s</td></tr>`, comma(depthOOB), status(depthOOB == 0))
	fmt.Fprintf(&b, `<tr><td>源深度缺失像素（背墙 dropout）</td><td class="mono">%s</td><td>保持空洞，不补采</td></tr>`, comma(sourceNoDepth))
	fmt.Fprintf(&b, `<tr><td>源像素投影越出目标画面（两场景合计）</td><td class="mono">%s</td><td>丢弃，不在边缘回采</td></tr>`, comma(sourceOff))
	b.WriteString(`</tbody></table></section>`)
	return b.String()
}

func totalRotationPixels(data Data) int {
	n := 0
	for i := range data.Cases {
		n += len(data.Cases[i].Rotation.DepthValid)
	}
	return n
}

func escape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}
