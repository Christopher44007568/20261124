"use strict";

const state = {
  scenes: [],
  report: null,
};

const $ = (id) => document.getElementById(id);

document.addEventListener("DOMContentLoaded", boot);

async function boot() {
  try {
    const response = await fetch("/api/scenes", { headers: { Accept: "application/json" } });
    if (!response.ok) throw new Error(`场景列表加载失败 (${response.status})`);
    state.scenes = await response.json();
    renderTabs();
    const requested = new URLSearchParams(location.search).get("scene");
    const initial = state.scenes.some((scene) => scene.id === requested)
      ? requested
      : state.scenes[0].id;
    await loadScene(initial);
  } catch (error) {
    showFatal(error);
  }
}

function renderTabs() {
  const tabs = $("scene-tabs");
  tabs.replaceChildren();
  for (const scene of state.scenes) {
    const button = document.createElement("button");
    button.type = "button";
    button.textContent = scene.title;
    button.dataset.scene = scene.id;
    button.addEventListener("click", () => loadScene(scene.id));
    tabs.append(button);
  }
}

async function loadScene(id) {
  setActiveTab(id);
  $("independence").textContent = "逐眼分析中…";
  $("independence").classList.remove("bad");
  try {
    const response = await fetch(`/api/report?scene=${encodeURIComponent(id)}`, {
      headers: { Accept: "application/json" },
    });
    if (!response.ok) {
      const body = await response.json().catch(() => ({}));
      throw new Error(body.error || `报告加载失败 (${response.status})`);
    }
    state.report = await response.json();
    history.replaceState(null, "", `?scene=${encodeURIComponent(id)}`);
    $("json-link").href = `/api/report?scene=${encodeURIComponent(id)}`;
    renderReport(state.report);
  } catch (error) {
    showFatal(error);
  }
}

function setActiveTab(id) {
  for (const button of document.querySelectorAll("#scene-tabs button")) {
    button.setAttribute("aria-pressed", String(button.dataset.scene === id));
  }
}

function renderReport(report) {
  $("scene-title").textContent = report.title;
  $("scene-description").textContent = report.description;
  $("generated-note").textContent =
    `每眼签名 ${report.eyes[0].signature.slice(0, 8)} / ${report.eyes[1].signature.slice(0, 8)}`;

  const independent =
    report.comparison.distinctNearCenters &&
    report.comparison.distinctClipResults &&
    report.comparison.distinctCoverage;
  const status = $("independence");
  status.textContent = independent ? "逐眼独立：已校验" : "逐眼结果异常相同";
  status.classList.toggle("bad", !independent);

  renderComparison(report);
  renderEye(report, 0);
  renderEye(report, 1);
  renderObjectTables(report);
  renderNotes(report);
}

function renderComparison(report) {
  const comparison = report.comparison;
  const coverageDelta = report.eyes[0].totals.coveragePct - report.eyes[1].totals.coveragePct;
  $("delta-coverage").textContent = formatSigned(coverageDelta, "个百分点");
  $("delta-area").textContent = formatSigned(comparison.clippedAreaDelta3D, "m²", 5);
  $("delta-edge").textContent = formatSigned(comparison.nearEdgeDeltaPx, "px");
  const distance = Math.hypot(
    report.eyes[0].nearPlaneCenter.x - report.eyes[1].nearPlaneCenter.x,
    report.eyes[0].nearPlaneCenter.y - report.eyes[1].nearPlaneCenter.y,
    report.eyes[0].nearPlaneCenter.z - report.eyes[1].nearPlaneCenter.z,
  );
  $("delta-center").textContent = `${(distance * 1000).toFixed(3)} mm`;
}

function renderEye(report, index) {
  const eye = report.eyes[index];
  const side = index === 0 ? "left" : "right";
  const canvas = $(`canvas-${side}`);
  drawEye(canvas, eye);

  $(`coverage-${side}`).textContent = `${eye.totals.coveragePct.toFixed(2)}% 覆盖`;
  const camera = eye.camera;
  $(`camera-${side}`).textContent =
    `pos (${camera.position.x.toFixed(3)}, ${camera.position.y.toFixed(3)}, ${camera.position.z.toFixed(3)}) · ` +
    `near ${camera.near.toFixed(3)} · ` +
    `frustum [${camera.frustum.left.toFixed(3)}, ${camera.frustum.right.toFixed(3)}] × ` +
    `[${camera.frustum.bottom.toFixed(3)}, ${camera.frustum.top.toFixed(3)}]`;

  const metrics = eye.totals;
  $(`metrics-${side}`).replaceChildren(...[
    metric("源三角形", formatInt(metrics.sourceTriangles)),
    metric("近面拒绝", formatInt(metrics.rejectedTriangles), metrics.rejectedTriangles > 0 ? "warn" : ""),
    metric("近裁后三角形", formatInt(metrics.nearClippedTriangles), "good"),
    metric("可见源三角形", formatInt(metrics.visibleSourceTriangles)),
    metric("可见屏幕三角形", formatInt(metrics.visibleScreenTriangles)),
    metric("裁剪前面积", `${metrics.sourceArea3D.toFixed(5)} m²`),
    metric("裁剪后面积", `${metrics.clippedArea3D.toFixed(5)} m²`),
    metric("近面边长度", `${metrics.nearEdgeLengthPx.toFixed(2)} px`, metrics.nearEdgeLengthPx > 0 ? "warn" : ""),
  ]);
}

function metric(label, value, tone = "") {
  const box = document.createElement("div");
  box.className = `metric ${tone}`.trim();
  const name = document.createElement("span");
  name.textContent = label;
  const strong = document.createElement("strong");
  strong.textContent = value;
  box.append(name, strong);
  return box;
}

function drawEye(canvas, eye) {
  const width = eye.camera.viewportWidth;
  const height = eye.camera.viewportHeight;
  if (canvas.width !== width || canvas.height !== height) {
    canvas.width = width;
    canvas.height = height;
  }
  const ctx = canvas.getContext("2d");
  ctx.clearRect(0, 0, width, height);
  ctx.fillStyle = "#070c14";
  ctx.fillRect(0, 0, width, height);
  drawGrid(ctx, width, height);

  for (const object of eye.objects) {
    const rgb = hexToRGB(object.material.color);
    const opacity = Number.isFinite(object.material.opacity) ? object.material.opacity : 1;
    for (const polygon of object.polygons) {
      if (polygon.screenPolygon.length < 3) continue;
      tracePolygon(ctx, polygon.screenPolygon);
      ctx.fillStyle = `rgba(${rgb}, ${0.1 + opacity * 0.16})`;
      ctx.fill();
      ctx.strokeStyle = `rgba(${rgb}, ${0.45 + opacity * 0.4})`;
      ctx.lineWidth = 1;
      ctx.stroke();
    }
  }

  for (const object of eye.objects) {
    for (const polygon of object.polygons) {
      for (const edge of polygon.nearEdges) {
        ctx.beginPath();
        ctx.moveTo(edge[0].x, edge[0].y);
        ctx.lineTo(edge[1].x, edge[1].y);
        ctx.strokeStyle = "#ff6b7a";
        ctx.lineWidth = 3;
        ctx.lineCap = "round";
        ctx.stroke();
      }
    }
  }

  ctx.strokeStyle = "rgba(80, 227, 194, 0.8)";
  ctx.lineWidth = 2;
  ctx.strokeRect(1, 1, width - 2, height - 2);
  drawHUD(ctx, width, height, eye);
}

function drawGrid(ctx, width, height) {
  ctx.save();
  ctx.strokeStyle = "rgba(70, 94, 128, 0.19)";
  ctx.lineWidth = 1;
  const step = 40;
  ctx.beginPath();
  for (let x = step; x < width; x += step) {
    ctx.moveTo(x + 0.5, 0);
    ctx.lineTo(x + 0.5, height);
  }
  for (let y = step; y < height; y += step) {
    ctx.moveTo(0, y + 0.5);
    ctx.lineTo(width, y + 0.5);
  }
  ctx.stroke();
  ctx.strokeStyle = "rgba(138, 151, 255, 0.3)";
  ctx.beginPath();
  ctx.moveTo(width / 2, 0);
  ctx.lineTo(width / 2, height);
  ctx.moveTo(0, height / 2);
  ctx.lineTo(width, height / 2);
  ctx.stroke();
  ctx.restore();
}

function drawHUD(ctx, width, height, eye) {
  const text = `${eye.eye.toUpperCase()} · REJECT ${eye.totals.rejectedTriangles}` +
    ` · COVER ${eye.totals.coveragePct.toFixed(2)}%`;
  ctx.save();
  ctx.font = "600 12px ui-monospace, SFMono-Regular, Menlo, monospace";
  const textWidth = ctx.measureText(text).width;
  ctx.fillStyle = "rgba(7, 12, 20, 0.82)";
  ctx.fillRect(10, height - 34, textWidth + 18, 24);
  ctx.fillStyle = "#c7d3e6";
  ctx.fillText(text, 19, height - 18);
  ctx.restore();
}

function tracePolygon(ctx, points) {
  ctx.beginPath();
  ctx.moveTo(points[0].x, points[0].y);
  for (let i = 1; i < points.length; i++) {
    ctx.lineTo(points[i].x, points[i].y);
  }
  ctx.closePath();
}

function renderObjectTables(report) {
  const container = $("object-tables");
  container.replaceChildren(...report.eyes.map((eye) => objectTable(eye)));
}

function objectTable(eye) {
  const section = document.createElement("div");
  section.className = "object-eye";
  const heading = document.createElement("h3");
  heading.textContent = `${eye.eye.toUpperCase()} · 对象明细`;
  const scroll = document.createElement("div");
  scroll.className = "table-scroll";
  const table = document.createElement("table");
  const head = document.createElement("thead");
  const headRow = document.createElement("tr");
  for (const label of ["对象", "源△", "拒绝△", "拒绝对象", "近裁△", "可见△", "源面积", "裁剪面积", "覆盖"]) {
    const th = document.createElement("th");
    th.textContent = label;
    headRow.append(th);
  }
  head.append(headRow);
  const body = document.createElement("tbody");
  for (const object of eye.objects) {
    body.append(objectRow(
      object.material.transparent ? `${object.mesh} (透明)` : object.mesh,
      object.metrics,
    ));
  }
  body.append(objectRow("合计", eye.totals, true));
  table.append(head, body);
  scroll.append(table);
  section.append(heading, scroll);
  return section;
}

function objectRow(name, metrics, isTotal = false) {
  const row = document.createElement("tr");
  if (isTotal) row.className = "total-row";
  const values = [
    name,
    formatInt(metrics.sourceTriangles),
    formatInt(metrics.rejectedTriangles),
    formatInt(metrics.rejectedObjects),
    formatInt(metrics.nearClippedTriangles),
    formatInt(metrics.visibleSourceTriangles),
    metrics.sourceArea3D.toFixed(5),
    metrics.clippedArea3D.toFixed(5),
    `${metrics.coveragePct.toFixed(2)}%`,
  ];
  values.forEach((value, index) => {
    const cell = document.createElement("td");
    cell.textContent = value;
    if (index === 0) cell.className = "name";
    row.append(cell);
  });
  return row;
}

function renderNotes(report) {
  const list = $("scene-notes");
  list.replaceChildren();
  const notes = [
    ...report.notes,
    `左眼覆盖 ${report.eyes[0].totals.coveragePct.toFixed(2)}%，` +
      `右眼覆盖 ${report.eyes[1].totals.coveragePct.toFixed(2)}%。`,
    `近面拒绝实例：左眼 ${report.eyes[0].totals.rejectedObjects}，` +
      `右眼 ${report.eyes[1].totals.rejectedObjects}。`,
  ];
  for (const text of notes) {
    const item = document.createElement("li");
    item.textContent = text;
    list.append(item);
  }
}

function hexToRGB(hex) {
  const clean = hex.replace("#", "");
  if (!/^[0-9a-fA-F]{6}$/.test(clean)) return "124, 140, 255";
  return [
    parseInt(clean.slice(0, 2), 16),
    parseInt(clean.slice(2, 4), 16),
    parseInt(clean.slice(4, 6), 16),
  ].join(", ");
}

function formatInt(value) {
  return new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 0 }).format(value);
}

function formatSigned(value, unit, digits = 2) {
  const sign = value > 0 ? "+" : "";
  return `${sign}${value.toFixed(digits)} ${unit}`;
}

function showFatal(error) {
  const status = $("independence");
  status.textContent = "分析失败";
  status.classList.add("bad");
  $("scene-description").textContent = "";
  const box = document.createElement("div");
  box.className = "error";
  box.textContent = error instanceof Error ? error.message : String(error);
  const target = $("metrics-left");
  if (target) target.replaceChildren(box);
  console.error(error);
}
