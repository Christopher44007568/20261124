package report

import (
	"fmt"
	"strings"

	"timewarp/metrics"
)

const reportCSS = `
:root{color-scheme:dark;--bg:#080b11;--panel:#101621;--panel2:#0c111a;--line:#263244;--text:#dbe3ef;--muted:#8c9bb0;--cyan:#32d8e8;--yellow:#ffd84a;--pink:#e340c8;--red:#ed4034;--orange:#ff9d1c}
*{box-sizing:border-box}body{margin:0;background:radial-gradient(1200px 500px at 50% -200px,#172233 0%,var(--bg) 60%);color:var(--text);font:14px/1.55 ui-sans-serif,system-ui,-apple-system,"Segoe UI",sans-serif}
main{max-width:1500px;margin:0 auto;padding:38px 28px 72px}h1{font-size:clamp(30px,4vw,54px);line-height:1.05;margin:8px 0 12px;letter-spacing:-.03em}
h2{font-size:24px;margin:0 0 8px;letter-spacing:-.01em}h3{font-size:15px;margin:22px 0 10px;color:#eef3fb}
p{color:var(--muted);margin:8px 0}.lead{font-size:17px;max-width:980px}.kick{font:600 11px/1.2 ui-monospace,monospace;letter-spacing:.18em;text-transform:uppercase;color:var(--cyan)}
.chips{display:flex;flex-wrap:wrap;gap:8px;margin:18px 0 0}.chip{border:1px solid var(--line);background:#0d141f;color:#aab8cb;border-radius:999px;padding:5px 10px;font:11px/1.2 ui-monospace,monospace}
.cards{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px;margin:26px 0}.card{background:var(--panel);border:1px solid var(--line);border-radius:14px;padding:14px}
.card b{display:block;font-size:22px;color:#fff;letter-spacing:-.02em}.card span{font-size:11px;color:var(--muted);text-transform:uppercase;letter-spacing:.08em}.card small{display:block;color:#75859a;font-size:11px;margin-top:2px}
.callout{background:linear-gradient(135deg,#101a28,#0c111a);border:1px solid var(--line);border-left:3px solid var(--cyan);border-radius:12px;padding:16px 18px;margin:18px 0}
.callout.warn{border-left-color:var(--orange)}.callout.good{border-left-color:#46d68c}
section.case{border-top:1px solid var(--line);margin-top:46px;padding-top:34px}.casehead{display:flex;gap:18px;align-items:flex-start;justify-content:space-between;flex-wrap:wrap}
.grid3{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px}.grid2{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}
figure{margin:0;background:var(--panel);border:1px solid var(--line);border-radius:12px;overflow:hidden}figure img{display:block;width:100%;height:auto;background:#06080c}figcaption{padding:9px 11px;font-size:12px;color:var(--muted);display:flex;justify-content:space-between;gap:8px}
figcaption b{color:#e9eff8;font-weight:600}.tag{font:10px/1.5 ui-monospace,monospace;color:#8fa0b7;border:1px solid #28374b;border-radius:4px;padding:0 5px;white-space:nowrap}
table{width:100%;border-collapse:collapse;background:var(--panel);border:1px solid var(--line);border-radius:10px;overflow:hidden}
th,td{padding:8px 10px;text-align:right;border-bottom:1px solid #1d2735;font-variant-numeric:tabular-nums}th:first-child,td:first-child{text-align:left}th{font-size:11px;text-transform:uppercase;letter-spacing:.08em;color:#93a3b9;background:#0d131d}tr:last-child td{border-bottom:0}
td.rot{color:#ffb0a6}td.dep{color:#9ce9f2}.mono{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}
.legend{display:flex;flex-wrap:wrap;gap:12px;align-items:center;margin:12px 0;color:#9ba9bc;font-size:12px}.sw{display:inline-block;width:12px;height:12px;border-radius:3px;vertical-align:-2px;margin-right:5px}
.sourcegrid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px;margin:16px 0}
.focus{margin:8px 0 4px}.focus figcaption{justify-content:flex-start;gap:18px}.focus figcaption span::before{content:"";display:inline-block;width:8px;height:8px;border-radius:2px;background:var(--cyan);margin-right:6px}
.foot{margin-top:52px;border-top:1px solid var(--line);padding-top:20px;color:#74839a;font-size:12px}
code{background:#0b111b;border:1px solid #223047;border-radius:4px;padding:1px 5px;color:#c8d5e7}
@media(max-width:1050px){.cards{grid-template-columns:repeat(2,1fr)}.grid3,.sourcegrid{grid-template-columns:1fr}.grid2{grid-template-columns:1fr}}
`

func renderHTML(data Data) string {
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>旋转 timewarp vs 深度重投影</title><style>`)
	b.WriteString(reportCSS)
	b.WriteString(`</style></head><body><main>`)
	b.WriteString(`<div class="kick">timewarp diagnostics / go standard library only</div>`)
	b.WriteString(`<h1>纯旋转 warp 与深度平移重投影</h1>`)
	b.WriteString(`<p class="lead">同一张带深度（局部缺失）的源帧，分别做纯旋转映射和带深度平移重投影。报告并排检查快速转头、头部前移两种运动下的房间、近杆、静止高光、拒绝掩码、误差热图与越界采样。</p>`)
	fmt.Fprintf(&b, `<div class="chips"><span class="chip">%d × %d</span><span class="chip">FOV-Y %.1f°</span><span class="chip">source depth dropout: back wall patch</span><span class="chip">zero external dependencies</span></div>`,
		data.Source.W, data.Source.H, data.FOVYDeg)
	b.WriteString(overviewHTML(data))
	b.WriteString(sourceHTML(data))
	for i := range data.Cases {
		b.WriteString(caseHTML(data.Cases[i], i+1))
	}
	b.WriteString(invariantHTML(data))
	b.WriteString(`<section class="case"><h2>约束与读图方式</h2><div class="grid2">`)
	b.WriteString(`<div class="callout good"><b>旋转路径不伪造深度。</b><p>纯旋转 warp 只使用两个姿态的相对旋转，输出 <code>DepthValid=false</code>（100%）。它也无法通过深度区分被遮挡表面，因此整帧颜色都按无限远旋转来映射。</p></div>`)
	b.WriteString(`<div class="callout warn"><b>缺深度不越界补采。</b><p>深度路径对没有源深度的像素不做投影；目标是空洞并进入拒绝掩码，而不是用边界像素补洞。所有源采样坐标都先做帧内检查，越界直接拒绝。</p></div>`)
	b.WriteString(`</div><h3>颜色图例</h3><div class="legend">`)
	b.WriteString(`<span><i class="sw" style="background:#0b1018"></i>有效</span><span><i class="sw" style="background:#ed4034"></i>越界采样</span><span><i class="sw" style="background:#dd40c6"></i>无覆盖 / 遮挡空洞</span><span><i class="sw" style="background:#ff9d1c"></i>相机后方</span>`)
	b.WriteString(`<span><i class="sw" style="background:#e22fcb"></i>源深度缺失</span><span><i class="sw" style="background:#32d8e8"></i>近杆区域</span><span><i class="sw" style="background:#ffd736"></i>静止高光</span>`)
	b.WriteString(`</div><p>误差热图：深蓝近 0，青色约 0.04，黄色约 0.09，红色约 0.22；灰黑斜纹表示该像素被拒绝，不参与 MAE。</p></section>`)
	b.WriteString(`<div class="foot">生成文件：report.html、metrics.json、source*.png、每个场景的 warp / mask / heat / focus PNG。所有图像由 Go 标准库直接编码。</div>`)
	b.WriteString(`</main></body></html>`)
	return b.String()
}

func overviewHTML(data Data) string {
	var b strings.Builder
	if len(data.Cases) == 0 {
		return ""
	}
	rotTurn, depTurn := data.Cases[0].RotStats, data.Cases[0].DepthStat
	rotDolly, depDolly := rotTurn, depTurn
	if len(data.Cases) > 1 {
		rotDolly, depDolly = data.Cases[1].RotStats, data.Cases[1].DepthStat
	}
	b.WriteString(`<div class="cards">`)
	fmt.Fprintf(&b, `<div class="card"><span>turn · rotation OOB</span><b>%s</b><small>rejected rays</small></div>`, comma(rotTurn.OutOfBounds))
	fmt.Fprintf(&b, `<div class="card"><span>turn · depth coverage</span><b>%s</b><small>valid target pixels</small></div>`, pct(depTurn.Coverage))
	fmt.Fprintf(&b, `<div class="card"><span>dolly · rotation MAE</span><b>%.2f</b><small>near pole / 0–255</small></div>`, rotDolly.MAE)
	fmt.Fprintf(&b, `<div class="card"><span>dolly · depth MAE</span><b>%.2f</b><small>near pole / 0–255</small></div>`, depDolly.MAE)
	b.WriteString(`</div><div class="callout"><b>结论</b><p>`)
	fmt.Fprintf(&b, `快速转头时旋转映射本身正确：旋转路径 MAE %.2f，深度路径 MAE %.2f；主要差别是两者在帧边缘都留下拒绝区。前移 0.68 单位时纯旋转没有视差：近杆区域 MAE %.2f（覆盖 %s），深度重投影为 %.2f（覆盖 %s）。旋转路径越界拒绝 %s 个采样，深度路径越界采样 0 次，另有 %s 个源投影落到目标画面外。`,
		rotTurn.MAE, depTurn.MAE,
		regionMAE(rotDolly, "near pole"), pct(regionCoverage(rotDolly, "near pole")),
		regionMAE(depDolly, "near pole"), pct(regionCoverage(depDolly, "near pole")),
		comma(rotTurn.OutOfBounds), comma(depDolly.SourceOffscreen))
	b.WriteString(`</p></div>`)
	return b.String()
}

func sourceHTML(data Data) string {
	return `<section class="case"><h2>源帧与诊断底图</h2><p>房间带有世界坐标网格、圆环和立柱；近杆在前景制造深度不连续；三个静止高光是世界锚定的亮点，不是屏幕空间 bloom。右上背墙的品红色块表示有颜色、无深度。</p><div class="sourcegrid">` +
		`<figure><img src="source.png"><figcaption><b>源颜色</b><span class="tag">pose 0</span></figcaption></figure>` +
		`<figure><img src="source_depth.png"><figcaption><b>源深度</b><span class="tag">warm = near</span></figcaption></figure>` +
		`<figure><img src="source_regions.png"><figcaption><b>语义区域 / 缺深度</b><span class="tag">magenta = invalid depth</span></figcaption></figure>` +
		`</div></section>`
}

func regionMAE(s metrics.Summary, name string) float64 {
	for _, r := range s.Regions {
		if r.Name == name {
			return r.MAE
		}
	}
	return 0
}

func regionCoverage(s metrics.Summary, name string) float64 {
	for _, r := range s.Regions {
		if r.Name == name {
			return r.Coverage
		}
	}
	return 0
}

func pct(v float64) string { return fmt.Sprintf("%.1f%%", v*100) }
func comma(v int) string   { return fmt.Sprintf("%d", v) }
