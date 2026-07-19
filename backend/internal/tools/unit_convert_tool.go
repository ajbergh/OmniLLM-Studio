package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// UnitConvertTool performs deterministic conversions across common unit families.
type UnitConvertTool struct{}

func NewUnitConvertTool() *UnitConvertTool { return &UnitConvertTool{} }

func (t *UnitConvertTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:             "unit_convert",
		Description:      "Convert values between common length, mass, volume, temperature, area, speed, and digital-storage units.",
		Category:         "utility",
		Enabled:          true,
		Version:          "2",
		Risk:             RiskLow,
		ReadOnly:         true,
		SupportsParallel: true,
		DefaultTimeoutMS: 2000,
		MaxResultBytes:   16384,
		Parameters: json.RawMessage(`{
			"type":"object",
			"required":["value","from","to"],
			"properties":{
				"value":{"type":"number"},
				"from":{"type":"string"},
				"to":{"type":"string"},
				"precision":{"type":"integer","minimum":0,"maximum":12,"default":6}
			}
		}`),
		OutputSchema: json.RawMessage(`{
			"type":"object",
			"required":["value","from","to","result"],
			"properties":{
				"value":{"type":"number"},
				"from":{"type":"string"},
				"to":{"type":"string"},
				"result":{"type":"number"},
				"dimension":{"type":"string"}
			}
		}`),
		Examples: []ToolExample{
			{Description: "Cups to milliliters", Arguments: json.RawMessage(`{"value":7,"from":"cups","to":"ml"}`)},
			{Description: "Miles to kilometers", Arguments: json.RawMessage(`{"value":5,"from":"mi","to":"km"}`)},
		},
	}
}

type unitConvertArgs struct {
	Value     float64 `json:"value"`
	From      string  `json:"from"`
	To        string  `json:"to"`
	Precision int     `json:"precision"`
}

type unitSpec struct {
	dimension string
	toBase    func(float64) float64
	fromBase  func(float64) float64
	canonical string
}

var linear = func(dimension, canonical string, factor float64) unitSpec {
	return unitSpec{
		dimension: dimension,
		canonical: canonical,
		toBase:    func(v float64) float64 { return v * factor },
		fromBase:  func(v float64) float64 { return v / factor },
	}
}

var unitSpecs = map[string]unitSpec{
	// Length base: meter.
	"m":          linear("length", "m", 1),
	"meter":      linear("length", "m", 1),
	"meters":     linear("length", "m", 1),
	"km":         linear("length", "km", 1000),
	"kilometer":  linear("length", "km", 1000),
	"kilometers": linear("length", "km", 1000),
	"cm":         linear("length", "cm", 0.01),
	"mm":         linear("length", "mm", 0.001),
	"in":         linear("length", "in", 0.0254),
	"inch":       linear("length", "in", 0.0254),
	"inches":     linear("length", "in", 0.0254),
	"ft":         linear("length", "ft", 0.3048),
	"foot":       linear("length", "ft", 0.3048),
	"feet":       linear("length", "ft", 0.3048),
	"yd":         linear("length", "yd", 0.9144),
	"yard":       linear("length", "yd", 0.9144),
	"yards":      linear("length", "yd", 0.9144),
	"mi":         linear("length", "mi", 1609.344),
	"mile":       linear("length", "mi", 1609.344),
	"miles":      linear("length", "mi", 1609.344),

	// Mass base: kilogram.
	"kg":        linear("mass", "kg", 1),
	"kilogram":  linear("mass", "kg", 1),
	"kilograms": linear("mass", "kg", 1),
	"g":         linear("mass", "g", 0.001),
	"gram":      linear("mass", "g", 0.001),
	"grams":     linear("mass", "g", 0.001),
	"mg":        linear("mass", "mg", 0.000001),
	"lb":        linear("mass", "lb", 0.45359237),
	"lbs":       linear("mass", "lb", 0.45359237),
	"pound":     linear("mass", "lb", 0.45359237),
	"pounds":    linear("mass", "lb", 0.45359237),
	"oz":        linear("mass", "oz", 0.028349523125),
	"ounce":     linear("mass", "oz", 0.028349523125),
	"ounces":    linear("mass", "oz", 0.028349523125),

	// Volume base: liter. US customary definitions.
	"l":           linear("volume", "L", 1),
	"liter":       linear("volume", "L", 1),
	"liters":      linear("volume", "L", 1),
	"litre":       linear("volume", "L", 1),
	"litres":      linear("volume", "L", 1),
	"ml":          linear("volume", "mL", 0.001),
	"milliliter":  linear("volume", "mL", 0.001),
	"milliliters": linear("volume", "mL", 0.001),
	"tsp":         linear("volume", "tsp", 0.00492892159375),
	"teaspoon":    linear("volume", "tsp", 0.00492892159375),
	"teaspoons":   linear("volume", "tsp", 0.00492892159375),
	"tbsp":        linear("volume", "tbsp", 0.01478676478125),
	"tablespoon":  linear("volume", "tbsp", 0.01478676478125),
	"tablespoons": linear("volume", "tbsp", 0.01478676478125),
	"cup":         linear("volume", "cup", 0.2365882365),
	"cups":        linear("volume", "cup", 0.2365882365),
	"floz":        linear("volume", "fl oz", 0.0295735295625),
	"fluidounce":  linear("volume", "fl oz", 0.0295735295625),
	"pint":        linear("volume", "pt", 0.473176473),
	"pints":       linear("volume", "pt", 0.473176473),
	"quart":       linear("volume", "qt", 0.946352946),
	"quarts":      linear("volume", "qt", 0.946352946),
	"gallon":      linear("volume", "gal", 3.785411784),
	"gallons":     linear("volume", "gal", 3.785411784),

	// Area base: square meter.
	"m2":      linear("area", "m²", 1),
	"sqm":     linear("area", "m²", 1),
	"km2":     linear("area", "km²", 1_000_000),
	"cm2":     linear("area", "cm²", 0.0001),
	"ft2":     linear("area", "ft²", 0.09290304),
	"sqft":    linear("area", "ft²", 0.09290304),
	"acre":    linear("area", "acre", 4046.8564224),
	"acres":   linear("area", "acre", 4046.8564224),
	"hectare": linear("area", "ha", 10000),
	"ha":      linear("area", "ha", 10000),

	// Speed base: meters per second.
	"m/s":   linear("speed", "m/s", 1),
	"mps":   linear("speed", "m/s", 1),
	"km/h":  linear("speed", "km/h", 1.0/3.6),
	"kph":   linear("speed", "km/h", 1.0/3.6),
	"mph":   linear("speed", "mph", 0.44704),
	"knot":  linear("speed", "kn", 0.514444444444),
	"knots": linear("speed", "kn", 0.514444444444),

	// Digital storage base: byte.
	"b":     linear("digital_storage", "B", 1),
	"byte":  linear("digital_storage", "B", 1),
	"bytes": linear("digital_storage", "B", 1),
	"kb":    linear("digital_storage", "kB", 1000),
	"mb":    linear("digital_storage", "MB", 1_000_000),
	"gb":    linear("digital_storage", "GB", 1_000_000_000),
	"tb":    linear("digital_storage", "TB", 1_000_000_000_000),
	"kib":   linear("digital_storage", "KiB", 1024),
	"mib":   linear("digital_storage", "MiB", 1024*1024),
	"gib":   linear("digital_storage", "GiB", 1024*1024*1024),
	"tib":   linear("digital_storage", "TiB", 1024*1024*1024*1024),

	// Temperature base: Celsius.
	"c": {
		dimension: "temperature", canonical: "°C",
		toBase: func(v float64) float64 { return v }, fromBase: func(v float64) float64 { return v },
	},
	"celsius": {
		dimension: "temperature", canonical: "°C",
		toBase: func(v float64) float64 { return v }, fromBase: func(v float64) float64 { return v },
	},
	"f": {
		dimension: "temperature", canonical: "°F",
		toBase: func(v float64) float64 { return (v - 32) * 5 / 9 }, fromBase: func(v float64) float64 { return v*9/5 + 32 },
	},
	"fahrenheit": {
		dimension: "temperature", canonical: "°F",
		toBase: func(v float64) float64 { return (v - 32) * 5 / 9 }, fromBase: func(v float64) float64 { return v*9/5 + 32 },
	},
	"k": {
		dimension: "temperature", canonical: "K",
		toBase: func(v float64) float64 { return v - 273.15 }, fromBase: func(v float64) float64 { return v + 273.15 },
	},
	"kelvin": {
		dimension: "temperature", canonical: "K",
		toBase: func(v float64) float64 { return v - 273.15 }, fromBase: func(v float64) float64 { return v + 273.15 },
	},
}

func (t *UnitConvertTool) Validate(raw json.RawMessage) error {
	var args unitConvertArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	from, ok := lookupUnit(args.From)
	if !ok {
		return fmt.Errorf("unsupported source unit %q", args.From)
	}
	to, ok := lookupUnit(args.To)
	if !ok {
		return fmt.Errorf("unsupported destination unit %q", args.To)
	}
	if from.dimension != to.dimension {
		return fmt.Errorf("cannot convert %s to %s", from.dimension, to.dimension)
	}
	if args.Precision < 0 || args.Precision > 12 {
		return fmt.Errorf("precision must be between 0 and 12")
	}
	return nil
}

func (t *UnitConvertTool) Execute(_ context.Context, raw json.RawMessage) (*ToolResult, error) {
	var args unitConvertArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	from, _ := lookupUnit(args.From)
	to, _ := lookupUnit(args.To)
	precision := args.Precision
	if precision == 0 {
		precision = 6
	}
	result := to.fromBase(from.toBase(args.Value))
	factor := math.Pow10(precision)
	result = math.Round(result*factor) / factor
	structured, _ := json.Marshal(map[string]interface{}{
		"value":     args.Value,
		"from":      from.canonical,
		"to":        to.canonical,
		"result":    result,
		"dimension": from.dimension,
		"precision": precision,
	})
	return &ToolResult{
		Content:    fmt.Sprintf("%.*f %s = %.*f %s", precision, args.Value, from.canonical, precision, result, to.canonical),
		Structured: structured,
		Metadata: map[string]interface{}{
			"dimension": from.dimension,
		},
	}, nil
}

func lookupUnit(value string) (unitSpec, bool) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, " ", "")
	normalized = strings.ReplaceAll(normalized, "²", "2")
	normalized = strings.TrimPrefix(normalized, "°")
	spec, ok := unitSpecs[normalized]
	return spec, ok
}
