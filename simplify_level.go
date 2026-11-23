package main

func cloneMesh(mesh *Mesh) *Mesh {
	vertices := append([]Vec3(nil), mesh.Vertices...)
	triangles := append([]Triangle(nil), mesh.Triangles...)
	boundary := append([]bool(nil), mesh.Boundary...)
	return &Mesh{Name: mesh.Name, Vertices: vertices, Triangles: triangles, Boundary: boundary}
}

func simplifyMesh(source *Mesh, targetTriangles int, name string) *Mesh {
	mesh := cloneMesh(source)
	quadrics := calculateQuadrics(mesh)
	vertexFaces := buildVertexFaces(mesh)
	edgeFaces := buildEdgeFaces(mesh)

	for activeTriangleCount(mesh) > targetTriangles {
		best := chooseBestEdge(mesh, quadrics, vertexFaces, edgeFaces)
		if best == nil {
			break
		}

		if !canCollapse(mesh, best) {
			break
		}

		a := best.key.a
		b := best.key.b
		affectedVertices := collectAffectedVertices(mesh, vertexFaces, a, b)
		quadrics[a] = quadrics[a].Add(quadrics[b])
		mesh.Vertices[a] = best.target

		delete(vertexFaces, b)
		for faceIndex := range mesh.Triangles {
			triangle := &mesh.Triangles[faceIndex]
			if !triangle.Active {
				continue
			}
			if triangle.V0 == b {
				triangle.V0 = a
			}
			if triangle.V1 == b {
				triangle.V1 = a
			}
			if triangle.V2 == b {
				triangle.V2 = a
			}
			if triangle.V0 == triangle.V1 || triangle.V1 == triangle.V2 || triangle.V2 == triangle.V0 {
				triangle.Active = false
				continue
			}
		}
		rebuildVertexFaces(mesh, vertexFaces, affectedVertices)
		rebuildEdgeFaces(mesh, edgeFaces, affectedVertices)
	}

	mesh.Name = name
	mesh.Triangles = compactMesh(mesh)
	return mesh
}

func chooseBestEdge(mesh *Mesh, quadrics []Quadric, vertexFaces map[int]map[int]bool, edgeFaces map[edgeKey][]int) *collapseEdge {
	var best *collapseEdge
	seen := make(map[edgeKey]bool)
	for vertex, neighbors := range vertexFaces {
		if mesh.Boundary[vertex] {
			continue
		}
		for neighbor := range neighbors {
			if mesh.Boundary[neighbor] {
				continue
			}
			key := makeEdgeKey(vertex, neighbor)
			if seen[key] {
				continue
			}
			seen[key] = true
			faces := edgeFaces[key]
			if len(faces) < 2 {
				continue
			}
			target := mesh.Vertices[key.a].Add(mesh.Vertices[key.b]).Scale(0.5)
			cost := quadrics[key.a].Evaluate(target) + quadrics[key.b].Evaluate(target)
			edge := &collapseEdge{key: key, cost: cost, target: target, faces: faces}
			if !canCollapse(mesh, edge) {
				continue
			}
			if best == nil || edge.cost < best.cost {
				best = edge
			}
		}
	}
	return best
}

func buildVertexFaces(mesh *Mesh) map[int]map[int]bool {
	vertexFaces := make(map[int]map[int]bool)
	for _, triangle := range mesh.Triangles {
		if !triangle.Active {
			continue
		}
		for _, vertex := range [3]int{triangle.V0, triangle.V1, triangle.V2} {
			if vertexFaces[vertex] == nil {
				vertexFaces[vertex] = make(map[int]bool)
			}
			for _, neighbor := range [3]int{triangle.V0, triangle.V1, triangle.V2} {
				if neighbor != vertex {
					vertexFaces[vertex][neighbor] = true
				}
			}
		}
	}
	return vertexFaces
}

func buildEdgeFaces(mesh *Mesh) map[edgeKey][]int {
	edgeFaces := make(map[edgeKey][]int)
	for faceIndex, triangle := range mesh.Triangles {
		if !triangle.Active {
			continue
		}
		vertices := [3]int{triangle.V0, triangle.V1, triangle.V2}
		for i := 0; i < 3; i++ {
			key := makeEdgeKey(vertices[i], vertices[(i+1)%3])
			edgeFaces[key] = append(edgeFaces[key], faceIndex)
		}
	}
	return edgeFaces
}

func rebuildEdgeFaces(mesh *Mesh, edgeFaces map[edgeKey][]int, affected map[int]bool) {
	for key := range edgeFaces {
		if affected[key.a] || affected[key.b] {
			delete(edgeFaces, key)
		}
	}
	for faceIndex, triangle := range mesh.Triangles {
		if !triangle.Active {
			continue
		}
		vertices := [3]int{triangle.V0, triangle.V1, triangle.V2}
		if !affected[vertices[0]] && !affected[vertices[1]] && !affected[vertices[2]] {
			continue
		}
		for i := 0; i < 3; i++ {
			key := makeEdgeKey(vertices[i], vertices[(i+1)%3])
			edgeFaces[key] = append(edgeFaces[key], faceIndex)
		}
	}
}

func collectAffectedVertices(mesh *Mesh, vertexFaces map[int]map[int]bool, a, b int) map[int]bool {
	affected := map[int]bool{a: true, b: true}
	for neighbor := range vertexFaces[a] {
		affected[neighbor] = true
	}
	for neighbor := range vertexFaces[b] {
		affected[neighbor] = true
	}
	return affected
}

func rebuildVertexFaces(mesh *Mesh, vertexFaces map[int]map[int]bool, affected map[int]bool) {
	for vertex := range affected {
		delete(vertexFaces, vertex)
	}
	for _, triangle := range mesh.Triangles {
		for _, vertex := range [3]int{triangle.V0, triangle.V1, triangle.V2} {
			if !affected[vertex] {
				continue
			}
			if vertexFaces[vertex] == nil {
				vertexFaces[vertex] = make(map[int]bool)
			}
			for _, neighbor := range [3]int{triangle.V0, triangle.V1, triangle.V2} {
				if neighbor != vertex {
					vertexFaces[vertex][neighbor] = true
				}
			}
		}
	}
}

func canCollapse(mesh *Mesh, edge *collapseEdge) bool {
	if len(edge.faces) < 2 || mesh.Boundary[edge.key.a] || mesh.Boundary[edge.key.b] {
		return false
	}

	for _, faceIndex := range edge.faces {
		triangle := mesh.Triangles[faceIndex]
		oldNormal := triangleNormal([3]Vec3{
			mesh.Vertices[triangle.V0],
			mesh.Vertices[triangle.V1],
			mesh.Vertices[triangle.V2],
		})
		points := [3]Vec3{
			mesh.Vertices[triangle.V0],
			mesh.Vertices[triangle.V1],
			mesh.Vertices[triangle.V2],
		}
		for i, vertex := range [3]int{triangle.V0, triangle.V1, triangle.V2} {
			if vertex == edge.key.b {
				points[i] = edge.target
			}
		}
		if points[0].Sub(points[1]).Length() < 1e-9 ||
			points[1].Sub(points[2]).Length() < 1e-9 ||
			points[2].Sub(points[0]).Length() < 1e-9 {
			return false
		}
		newNormal := triangleNormal(points)
		if newNormal.Length() < 1e-12 || oldNormal.Dot(newNormal) < 0.998 {
			return false
		}
	}
	return true
}

func compactMesh(mesh *Mesh) []Triangle {
	activeTriangles := make([]Triangle, 0, activeTriangleCount(mesh))
	for _, triangle := range mesh.Triangles {
		if triangle.Active {
			triangle.Active = true
			activeTriangles = append(activeTriangles, triangle)
		}
	}
	mesh.Triangles = activeTriangles

	used := make([]bool, len(mesh.Vertices))
	for _, triangle := range mesh.Triangles {
		used[triangle.V0] = true
		used[triangle.V1] = true
		used[triangle.V2] = true
	}

	remap := make([]int, len(mesh.Vertices))
	for i := range remap {
		remap[i] = -1
	}

	nextVertices := make([]Vec3, 0)
	nextBoundary := make([]bool, 0)
	for oldIndex, isUsed := range used {
		if !isUsed {
			continue
		}
		remap[oldIndex] = len(nextVertices)
		nextVertices = append(nextVertices, mesh.Vertices[oldIndex])
		nextBoundary = append(nextBoundary, mesh.Boundary[oldIndex])
	}

	nextTriangles := make([]Triangle, len(mesh.Triangles))
	for i, triangle := range mesh.Triangles {
		nextTriangles[i] = Triangle{
			V0:     remap[triangle.V0],
			V1:     remap[triangle.V1],
			V2:     remap[triangle.V2],
			Active: true,
		}
	}

	mesh.Vertices = nextVertices
	mesh.Boundary = nextBoundary
	return nextTriangles
}

func activeTriangleCount(mesh *Mesh) int {
	count := 0
	for _, triangle := range mesh.Triangles {
		if triangle.Active {
			count++
		}
	}
	return count
}
