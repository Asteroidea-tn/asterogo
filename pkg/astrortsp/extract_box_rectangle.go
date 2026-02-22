package astrortsp

type Point struct {
	X, Y int
}

type Rectangle struct {
	X, Y, Width, Height int
}

func ExtractBoundingBox(p1, p2, p3, p4 Point) Rectangle {
	// Collect all points
	points := []Point{p1, p2, p3, p4}

	// Find min and max for x and y
	minX := points[0].X
	maxX := points[0].X
	minY := points[0].Y
	maxY := points[0].Y

	for _, p := range points[1:] {
		if p.X < minX {
			minX = p.X
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}

	return Rectangle{
		X:      minX,
		Y:      minY,
		Width:  maxX - minX,
		Height: maxY - minY,
	}
}
