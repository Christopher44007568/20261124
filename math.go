package binocular

import "math"

// Vec3 is a three-component vector.
type Vec3 struct {
	X, Y, Z float64
}

// Add returns v+other.
func (v Vec3) Add(other Vec3) Vec3 {
	return Vec3{v.X + other.X, v.Y + other.Y, v.Z + other.Z}
}

// Sub returns v-other.
func (v Vec3) Sub(other Vec3) Vec3 {
	return Vec3{v.X - other.X, v.Y - other.Y, v.Z - other.Z}
}

// Scale returns v*s.
func (v Vec3) Scale(s float64) Vec3 {
	return Vec3{v.X * s, v.Y * s, v.Z * s}
}

// Mul returns the component-wise product of v and other.
func (v Vec3) Mul(other Vec3) Vec3 {
	return Vec3{v.X * other.X, v.Y * other.Y, v.Z * other.Z}
}

func (v Vec3) dot(other Vec3) float64 {
	return v.X*other.X + v.Y*other.Y + v.Z*other.Z
}

func (v Vec3) cross(other Vec3) Vec3 {
	return Vec3{
		v.Y*other.Z - v.Z*other.Y,
		v.Z*other.X - v.X*other.Z,
		v.X*other.Y - v.Y*other.X,
	}
}

func (v Vec3) length() float64 {
	return math.Sqrt(v.dot(v))
}

func (v Vec3) normalized() Vec3 {
	n := v.length()
	if n == 0 {
		return Vec3{}
	}
	return v.Scale(1 / n)
}

// Vec4 is a homogeneous vector.
type Vec4 struct {
	X, Y, Z, W float64
}

// Mat4 is a row-major 4x4 matrix. Column vectors are transformed as m*v.
type Mat4 [16]float64

// Identity4 returns the identity matrix.
func Identity4() Mat4 {
	return Mat4{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
}

// Translate returns a translation matrix.
func Translate(x, y, z float64) Mat4 {
	m := Identity4()
	m[3], m[7], m[11] = x, y, z
	return m
}

// Scale returns a non-uniform scale matrix.
func Scale(x, y, z float64) Mat4 {
	return Mat4{
		x, 0, 0, 0,
		0, y, 0, 0,
		0, 0, z, 0,
		0, 0, 0, 1,
	}
}

// RotateX returns a right-handed rotation around the X axis.
func RotateX(radians float64) Mat4 {
	s, c := math.Sincos(radians)
	return Mat4{
		1, 0, 0, 0,
		0, c, -s, 0,
		0, s, c, 0,
		0, 0, 0, 1,
	}
}

// RotateY returns a right-handed rotation around the Y axis.
func RotateY(radians float64) Mat4 {
	s, c := math.Sincos(radians)
	return Mat4{
		c, 0, s, 0,
		0, 1, 0, 0,
		-s, 0, c, 0,
		0, 0, 0, 1,
	}
}

// RotateZ returns a right-handed rotation around the Z axis.
func RotateZ(radians float64) Mat4 {
	s, c := math.Sincos(radians)
	return Mat4{
		c, -s, 0, 0,
		s, c, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
}

// Mul returns m*other. With column vectors, other is applied first.
func (m Mat4) Mul(other Mat4) Mat4 {
	var out Mat4
	for row := 0; row < 4; row++ {
		for col := 0; col < 4; col++ {
			var sum float64
			for k := 0; k < 4; k++ {
				sum += m[row*4+k] * other[k*4+col]
			}
			out[row*4+col] = sum
		}
	}
	return out
}

// Transform applies the matrix to a homogeneous vector.
func (m Mat4) Transform(v Vec4) Vec4 {
	return Vec4{
		X: m[0]*v.X + m[1]*v.Y + m[2]*v.Z + m[3]*v.W,
		Y: m[4]*v.X + m[5]*v.Y + m[6]*v.Z + m[7]*v.W,
		Z: m[8]*v.X + m[9]*v.Y + m[10]*v.Z + m[11]*v.W,
		W: m[12]*v.X + m[13]*v.Y + m[14]*v.Z + m[15]*v.W,
	}
}

// Perspective returns an OpenGL-style perspective projection. Clip-space
// near and far planes are at z=-w and z=w respectively.
func Perspective(fovYRadians, aspect, near, far float64) Mat4 {
	f := 1 / math.Tan(fovYRadians/2)
	return Mat4{
		f / aspect, 0, 0, 0,
		0, f, 0, 0,
		0, 0, (far + near) / (near - far), 2 * far * near / (near - far),
		0, 0, -1, 0,
	}
}

// LookAt returns a right-handed view matrix.
func LookAt(eye, target, up Vec3) Mat4 {
	forward := target.Sub(eye).normalized()
	side := forward.cross(up).normalized()
	trueUp := side.cross(forward)
	return Mat4{
		side.X, side.Y, side.Z, -side.dot(eye),
		trueUp.X, trueUp.Y, trueUp.Z, -trueUp.dot(eye),
		-forward.X, -forward.Y, -forward.Z, forward.dot(eye),
		0, 0, 0, 1,
	}
}

// Fingerprint returns a stable 64-bit FNV-1a hash of a matrix's values.
func (m Mat4) Fingerprint() uint64 {
	const (
		offset64 = uint64(14695981039346656037)
		prime64  = uint64(1099511628211)
	)
	hash := offset64
	for _, value := range m {
		bits := math.Float64bits(value)
		for shift := 0; shift < 64; shift += 8 {
			hash ^= (bits >> shift) & 0xff
			hash *= prime64
		}
	}
	return hash
}
