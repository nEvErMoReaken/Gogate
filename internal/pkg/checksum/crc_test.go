package checksum

import (
	"testing"
)

func TestCalculateCRC16(t *testing.T) {
	tests := []struct {
		name       string
		polynomial uint16
		data       []byte
		want       uint16
	}{
		{
			name:       "empty data",
			polynomial: CRC16_MODBUS,
			data:       []byte{},
			want:       0xFFFF,
		},
		{
			name:       "single byte",
			polynomial: CRC16_MODBUS,
			data:       []byte{0x01},
			want:       0xC0F1,
		},
		{
			name:       "multiple bytes",
			polynomial: CRC16_MODBUS,
			data:       []byte{0x01, 0x02, 0x03, 0x04},
			want:       0xB6C9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateCRC16(tt.polynomial, tt.data); got != tt.want {
				t.Errorf("CalculateCRC16() = %x, want %x", got, tt.want)
			}
		})
	}
}

func TestCalculateCRC32(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want uint32
	}{
		{
			name: "empty data",
			data: []byte{},
			want: 0x0,
		},
		{
			name: "single byte",
			data: []byte{0x01},
			want: 0x77073096,
		},
		{
			name: "multiple bytes",
			data: []byte{0x01, 0x02, 0x03, 0x04},
			want: 0xB63CFBCD,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateCRC32(tt.data); got != tt.want {
				t.Errorf("CalculateCRC32() = %x, want %x", got, tt.want)
			}
		})
	}
}

func TestExtractUint16(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		byteOrder string
		want      uint16
	}{
		{
			name:      "big endian",
			data:      []byte{0x12, 0x34},
			byteOrder: "big",
			want:      0x1234,
		},
		{
			name:      "little endian",
			data:      []byte{0x12, 0x34},
			byteOrder: "little",
			want:      0x3412,
		},
		{
			name:      "insufficient data",
			data:      []byte{0x12},
			byteOrder: "big",
			want:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractUint16(tt.data, tt.byteOrder); got != tt.want {
				t.Errorf("ExtractUint16() = %x, want %x", got, tt.want)
			}
		})
	}
}

func TestExtractUint32(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		byteOrder string
		want      uint32
	}{
		{
			name:      "big endian",
			data:      []byte{0x12, 0x34, 0x56, 0x78},
			byteOrder: "big",
			want:      0x12345678,
		},
		{
			name:      "little endian",
			data:      []byte{0x12, 0x34, 0x56, 0x78},
			byteOrder: "little",
			want:      0x78563412,
		},
		{
			name:      "insufficient data",
			data:      []byte{0x12, 0x34},
			byteOrder: "big",
			want:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractUint32(tt.data, tt.byteOrder); got != tt.want {
				t.Errorf("ExtractUint32() = %x, want %x", got, tt.want)
			}
		})
	}
}
