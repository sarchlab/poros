package benchmarks

// Independent exhaustive search plus matrix-form Walsh-Hadamard transform.
func x264Reference(radius int) [3]int {
	width, origin := 128, 48
	if radius > 8 {
		width, origin = 2*radius+128, radius+48
	}
	current, reference := make([]byte, width*width), make([]byte, width*width)
	for i := range current {
		current[i] = byte(i*73 + 17)
		reference[i] = current[i]
	}
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			reference[(origin-2+y)*width+origin+3+x] = current[(origin+y)*width+origin+x] + byte((y*3+x*7)%5)
		}
	}
	abs := func(x int) int {
		if x < 0 {
			return -x
		}
		return x
	}
	best, mx, my := int(^uint(0)>>1), 0, 0
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			cost := 0
			for y := 0; y < 16; y++ {
				for x := 0; x < 16; x++ {
					cost += abs(int(current[(origin+y)*width+origin+x]) - int(reference[(origin+dy+y)*width+origin+dx+x]))
				}
			}
			if cost < best {
				best, mx, my = cost, dx, dy
			}
		}
	}
	// Orthogonal sign matrix, expressed without the guest butterfly sequence.
	h := [4][4]int{{1, 1, 1, 1}, {1, 1, -1, -1}, {1, -1, -1, 1}, {1, -1, 1, -1}}
	total := 0
	for by := 0; by < 16; by += 4 {
		for bx := 0; bx < 16; bx += 4 {
			sum := 0
			for i := 0; i < 4; i++ {
				for j := 0; j < 4; j++ {
					v := 0
					for y := 0; y < 4; y++ {
						for x := 0; x < 4; x++ {
							v += h[i][y] * h[j][x] * (int(current[(origin+by+y)*width+origin+bx+x]) - int(reference[(origin+my+by+y)*width+origin+mx+bx+x]))
						}
					}
					sum += abs(v)
				}
			}
			total += (sum + 1) / 2
		}
	}
	return [3]int{total, mx, my}
}
