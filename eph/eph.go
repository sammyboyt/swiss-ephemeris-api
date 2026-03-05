package eph

/*
#cgo CFLAGS: -I${SRCDIR}/sweph/src
#cgo LDFLAGS: -L${SRCDIR}/sweph/src -lswe -ldl -lm
#include "swephexp.h"

int init()
{
	// Set ephemeris path to /app where data files are located in containers
	swe_set_ephe_path("/app/");
	return 0;
}

int sweCalcUt(int jy, int jm, int jd, double jut, int p, double *xx)
{
	char snam[40], serr[AS_MAXCH];
	double tjd_ut, x2[6];
	long iflag, iflgret;
	iflag = SEFLG_SPEED;

	tjd_ut = swe_julday(jy, jm, jd, jut, SE_GREG_CAL);
	// printf("planet \tlongitude\tlatitude\tdistance\tspeed long.\n");

	iflgret = swe_calc_ut(tjd_ut, p, iflag, xx, serr);
	if (iflgret < 0) {
		printf("error: %s\n", serr);
		return -1;
	}

	// swe_get_planet_name(p, snam);
	// printf("%10s\t%11.7f\t%10.7f\t%10.7f\t%10.7f\n", snam, xx[0], xx[1], xx[2], xx[3]);

	return OK;
}

int sweHouses(int jy, int jm, int jd, double jut, double lat, double lng, char hsys,
	double *cusps, double* ascmc)
{
	double tjd_ut = swe_julday(jy, jm, jd, jut, SE_GREG_CAL);

	swe_houses(tjd_ut, lat, lng, hsys, cusps, ascmc);

	return OK;
}
*/
import "C"
import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
)

type key int

const ephPlanetKey key = 0
const ephHouseKey key = 1

// Swiss Ephemeris constants (subset needed for our implementation)
const (
	SE_SUN       = 0
	SE_MOON      = 1
	SE_MERCURY   = 2
	SE_VENUS     = 3
	SE_MARS      = 4
	SE_JUPITER   = 5
	SE_SATURN    = 6
	SE_URANUS    = 7
	SE_NEPTUNE   = 8
	SE_PLUTO     = 9
	SE_MEAN_NODE = 10
	SE_TRUE_NODE = 11
	SE_MEAN_APOG = 12
	SE_OSCU_APOG = 13
	SE_CHIRON    = 15
	SE_PHOLUS    = 16
	SE_CERES     = 17
	SE_PALLAS    = 18
	SE_JUNO      = 19
	SE_VESTA     = 20

	SEFLG_SWIEPH = 2
	SEFLG_SPEED  = 256

	SE_GREG_CAL = 1
)

// m ensures exclusive ephemeris interaction, preventing data racing.
var m sync.Mutex

// CelestialBodyType categorizes different astronomical objects
type CelestialBodyType string

const (
	TypePlanet    CelestialBodyType = "planet"     // Sun, Moon, 8 planets
	TypeNode      CelestialBodyType = "node"       // Lunar nodes, apogees
	TypeAsteroid  CelestialBodyType = "asteroid"   // Ceres, Pallas, Juno, Vesta
	TypeCentaur   CelestialBodyType = "centaur"    // Chiron, Pholus
	TypeFixedStar CelestialBodyType = "fixed_star" // Stars by constellation
)

// CelestialBody represents any astronomical object with full ephemeris data
type CelestialBody struct {
	ID            int               `json:"id"`
	Name          string            `json:"name"`
	Type          CelestialBodyType `json:"type"`
	Constellation *string           `json:"constellation,omitempty"` // For fixed stars

	// Position Data (J2000 ecliptic coordinates)
	Longitude float64 `json:"longitude"`   // 0-360°
	Latitude  float64 `json:"latitude"`    // Usually small for planets
	Distance  float64 `json:"distance_au"` // AU from Earth/Sun

	// Motion Data
	SpeedLongitude float64 `json:"speed_longitude"` // °/day
	SpeedLatitude  float64 `json:"speed_latitude"`  // °/day
	SpeedDistance  float64 `json:"speed_distance"`  // AU/day

	// Derived Properties
	Retrograde bool     `json:"retrograde"`          // speed_longitude < 0
	Magnitude  *float64 `json:"magnitude,omitempty"` // For fixed stars

	// Metadata
	Category string `json:"category"` // "traditional", "modern", "asteroid"
	Sequence int    `json:"sequence"` // Display order
}

// Constellation represents a star grouping
type Constellation struct {
	Name      string          `json:"name"`
	Abbrev    string          `json:"abbrev"` // 3-letter code (Tau, Sco, Leo)
	LatinName string          `json:"latin_name"`
	Stars     []CelestialBody `json:"stars"`
	StarCount int             `json:"star_count"`
}

// EphemerisConfig controls which bodies to calculate
type EphemerisConfig struct {
	// Traditional planets and luminaries
	IncludeTraditional bool `yaml:"include_traditional"`

	// Extended bodies
	IncludeNodes     bool `yaml:"include_nodes"`
	IncludeAsteroids bool `yaml:"include_asteroids"`
	IncludeCentaurs  bool `yaml:"include_centaurs"`

	// Fixed stars
	IncludeFixedStars bool     `yaml:"include_fixed_stars"`
	MaxStarMagnitude  float64  `yaml:"max_star_magnitude"` // Only stars brighter than this
	Constellations    []string `yaml:"constellations"`     // Which constellations to include

	// Custom body selection
	CustomBodies []int `yaml:"custom_bodies"` // Specific SE_ constants

	// Performance settings
	UseSpeed         bool  `yaml:"use_speed"`         // Include speed calculations
	CalculationFlags int32 `yaml:"calculation_flags"` // Swiss Ephemeris flags
}

// BodyDefinition maps IDs to metadata
type BodyDefinition struct {
	ID         int
	Name       string
	Type       CelestialBodyType
	Category   string
	Sequence   int
	SEConstant int32 // Swiss Ephemeris constant
}

// GetAvailableBodies returns all defined astronomical bodies
func GetAvailableBodies() []BodyDefinition {
	return []BodyDefinition{
		// Traditional Planets & Luminaries
		{ID: 0, Name: "Sun", Type: TypePlanet, Category: "traditional", Sequence: 1, SEConstant: SE_SUN},
		{ID: 1, Name: "Moon", Type: TypePlanet, Category: "traditional", Sequence: 2, SEConstant: SE_MOON},
		{ID: 2, Name: "Mercury", Type: TypePlanet, Category: "traditional", Sequence: 3, SEConstant: SE_MERCURY},
		{ID: 3, Name: "Venus", Type: TypePlanet, Category: "traditional", Sequence: 4, SEConstant: SE_VENUS},
		{ID: 4, Name: "Mars", Type: TypePlanet, Category: "traditional", Sequence: 5, SEConstant: SE_MARS},
		{ID: 5, Name: "Jupiter", Type: TypePlanet, Category: "traditional", Sequence: 6, SEConstant: SE_JUPITER},
		{ID: 6, Name: "Saturn", Type: TypePlanet, Category: "traditional", Sequence: 7, SEConstant: SE_SATURN},
		{ID: 7, Name: "Uranus", Type: TypePlanet, Category: "modern", Sequence: 8, SEConstant: SE_URANUS},
		{ID: 8, Name: "Neptune", Type: TypePlanet, Category: "modern", Sequence: 9, SEConstant: SE_NEPTUNE},
		{ID: 9, Name: "Pluto", Type: TypePlanet, Category: "dwarf", Sequence: 10, SEConstant: SE_PLUTO},

		// Lunar Nodes
		{ID: 10, Name: "Mean Node", Type: TypeNode, Category: "node", Sequence: 11, SEConstant: SE_MEAN_NODE},
		{ID: 11, Name: "True Node", Type: TypeNode, Category: "node", Sequence: 12, SEConstant: SE_TRUE_NODE},

		// Lunar Apogees (Lilith)
		{ID: 12, Name: "Mean Lilith", Type: TypeNode, Category: "lilith", Sequence: 13, SEConstant: SE_MEAN_APOG},
		{ID: 13, Name: "True Lilith", Type: TypeNode, Category: "lilith", Sequence: 14, SEConstant: SE_OSCU_APOG},

		// Centaurs
		{ID: 15, Name: "Chiron", Type: TypeCentaur, Category: "centaur", Sequence: 15, SEConstant: SE_CHIRON},
		{ID: 16, Name: "Pholus", Type: TypeCentaur, Category: "centaur", Sequence: 16, SEConstant: SE_PHOLUS},

		// Main Asteroids
		{ID: 17, Name: "Ceres", Type: TypeAsteroid, Category: "asteroid", Sequence: 17, SEConstant: SE_CERES},
		{ID: 18, Name: "Pallas", Type: TypeAsteroid, Category: "asteroid", Sequence: 18, SEConstant: SE_PALLAS},
		{ID: 19, Name: "Juno", Type: TypeAsteroid, Category: "asteroid", Sequence: 19, SEConstant: SE_JUNO},
		{ID: 20, Name: "Vesta", Type: TypeAsteroid, Category: "asteroid", Sequence: 20, SEConstant: SE_VESTA},
	}
}

// GetBodyByID finds a body definition by its ID
func GetBodyByID(id int) (BodyDefinition, bool) {
	bodies := GetAvailableBodies()
	for _, body := range bodies {
		if body.ID == id {
			return body, true
		}
	}
	return BodyDefinition{}, false
}

// GetBodyBySEConstant finds a body definition by its Swiss Ephemeris constant
func GetBodyBySEConstant(seConst int32) (BodyDefinition, bool) {
	bodies := GetAvailableBodies()
	for _, body := range bodies {
		if body.SEConstant == seConst {
			return body, true
		}
	}
	return BodyDefinition{}, false
}

// ConstellationDefinition defines constellation boundaries and metadata
type ConstellationDefinition struct {
	Name      string
	Abbrev    string // 3-letter abbreviation
	LatinName string
	RAStart   float64 // Right Ascension start (degrees)
	RAEnd     float64 // Right Ascension end (degrees)
	DecStart  float64 // Declination start (degrees)
	DecEnd    float64 // Declination end (degrees)
}

// ZodiacAbbrev is the special abbreviation for zodiac constellation aggregation
const ZodiacAbbrev = "Zodiac"

// getZodiacMemberConstellations returns all 12 traditional zodiac constellations
func getZodiacMemberConstellations() []string {
	return []string{
		"Ari", "Tau", "Gem", "Cnc", "Leo", "Vir",
		"Lib", "Sco", "Sgr", "Cap", "Aqr", "Psc",
	}
}

// isZodiacConstellation checks if an abbreviation represents a zodiac constellation
func isZodiacConstellation(abbrev string) bool {
	members := getZodiacMemberConstellations()
	for _, member := range members {
		if member == abbrev {
			return true
		}
	}
	return false
}

// ExpandZodiacConstellations expands "Zodiac" to all 12 zodiac constellations and removes duplicates
func ExpandZodiacConstellations(requested []string) []string {
	expanded := make([]string, 0, len(requested)*12)

	for _, req := range requested {
		if req == ZodiacAbbrev {
			// Expand Zodiac to all 12 member constellations
			expanded = append(expanded, getZodiacMemberConstellations()...)
		} else {
			expanded = append(expanded, req)
		}
	}

	// Remove duplicates while preserving order
	seen := make(map[string]bool)
	unique := make([]string, 0)
	for _, item := range expanded {
		if !seen[item] {
			seen[item] = true
			unique = append(unique, item)
		}
	}

	return unique
}

// GetAvailableConstellations returns all defined constellations
func GetAvailableConstellations() []ConstellationDefinition {
	return []ConstellationDefinition{
		// Special Zodiac meta-constellation
		{Name: "Zodiac", Abbrev: ZodiacAbbrev, LatinName: "Zodiac", RAStart: 0, RAEnd: 360, DecStart: -90, DecEnd: 90},

		// Traditional Zodiac constellations
		{Name: "Aries", Abbrev: "Ari", LatinName: "Aries", RAStart: 20, RAEnd: 70, DecStart: 10, DecEnd: 30},
		{Name: "Taurus", Abbrev: "Tau", LatinName: "Taurus", RAStart: 50, RAEnd: 110, DecStart: 0, DecEnd: 30},
		{Name: "Gemini", Abbrev: "Gem", LatinName: "Gemini", RAStart: 80, RAEnd: 130, DecStart: 10, DecEnd: 35},
		{Name: "Cancer", Abbrev: "Cnc", LatinName: "Cancer", RAStart: 120, RAEnd: 140, DecStart: 10, DecEnd: 35},
		{Name: "Leo", Abbrev: "Leo", LatinName: "Leo", RAStart: 140, RAEnd: 190, DecStart: 0, DecEnd: 35},
		{Name: "Virgo", Abbrev: "Vir", LatinName: "Virgo", RAStart: 180, RAEnd: 230, DecStart: -15, DecEnd: 15},
		{Name: "Libra", Abbrev: "Lib", LatinName: "Libra", RAStart: 220, RAEnd: 250, DecStart: -30, DecEnd: 0},
		{Name: "Scorpius", Abbrev: "Sco", LatinName: "Scorpius", RAStart: 240, RAEnd: 280, DecStart: -45, DecEnd: -15},
		{Name: "Sagittarius", Abbrev: "Sgr", LatinName: "Sagittarius", RAStart: 270, RAEnd: 320, DecStart: -45, DecEnd: -10},
		{Name: "Capricornus", Abbrev: "Cap", LatinName: "Capricornus", RAStart: 300, RAEnd: 330, DecStart: -30, DecEnd: -5},
		{Name: "Aquarius", Abbrev: "Aqr", LatinName: "Aquarius", RAStart: 320, RAEnd: 360, DecStart: -25, DecEnd: 5},
		{Name: "Pisces", Abbrev: "Psc", LatinName: "Pisces", RAStart: 0, RAEnd: 30, DecStart: -10, DecEnd: 15},

		// Other notable constellations
		{Name: "Ursa Major", Abbrev: "UMa", LatinName: "Ursa Major", RAStart: 130, RAEnd: 280, DecStart: 30, DecEnd: 75},
		{Name: "Ursa Minor", Abbrev: "UMi", LatinName: "Ursa Minor", RAStart: 210, RAEnd: 360, DecStart: 65, DecEnd: 90},
		{Name: "Orion", Abbrev: "Ori", LatinName: "Orion", RAStart: 70, RAEnd: 100, DecStart: -10, DecEnd: 25},
		{Name: "Lyra", Abbrev: "Lyr", LatinName: "Lyra", RAStart: 275, RAEnd: 295, DecStart: 25, DecEnd: 50},
		{Name: "Cygnus", Abbrev: "Cyg", LatinName: "Cygnus", RAStart: 290, RAEnd: 325, DecStart: 25, DecEnd: 60},
		{Name: "Pegasus", Abbrev: "Peg", LatinName: "Pegasus", RAStart: 320, RAEnd: 360, DecStart: 0, DecEnd: 40},
		{Name: "Andromeda", Abbrev: "And", LatinName: "Andromeda", RAStart: 0, RAEnd: 40, DecStart: 20, DecEnd: 55},
		{Name: "Cassiopeia", Abbrev: "Cas", LatinName: "Cassiopeia", RAStart: 0, RAEnd: 60, DecStart: 45, DecEnd: 75},
		{Name: "Perseus", Abbrev: "Per", LatinName: "Perseus", RAStart: 40, RAEnd: 90, DecStart: 30, DecEnd: 60},
		{Name: "Auriga", Abbrev: "Aur", LatinName: "Auriga", RAStart: 70, RAEnd: 110, DecStart: 25, DecEnd: 55},
		{Name: "Canis Major", Abbrev: "CMa", LatinName: "Canis Major", RAStart: 90, RAEnd: 120, DecStart: -35, DecEnd: -10},
		{Name: "Canis Minor", Abbrev: "CMi", LatinName: "Canis Minor", RAStart: 110, RAEnd: 130, DecStart: 0, DecEnd: 15},
		{Name: "Draco", Abbrev: "Dra", LatinName: "Draco", RAStart: 240, RAEnd: 360, DecStart: 45, DecEnd: 90},
		{Name: "Hercules", Abbrev: "Her", LatinName: "Hercules", RAStart: 240, RAEnd: 290, DecStart: 15, DecEnd: 55},
		{Name: "Bootes", Abbrev: "Boo", LatinName: "Bootes", RAStart: 210, RAEnd: 240, DecStart: 0, DecEnd: 55},
		{Name: "Corona Borealis", Abbrev: "CrB", LatinName: "Corona Borealis", RAStart: 230, RAEnd: 250, DecStart: 25, DecEnd: 40},
	}
}

// GetConstellationByAbbrev finds a constellation by its 3-letter abbreviation
func GetConstellationByAbbrev(abbrev string) (ConstellationDefinition, bool) {
	constellations := GetAvailableConstellations()
	for _, constell := range constellations {
		if constell.Abbrev == abbrev {
			return constell, true
		}
	}
	return ConstellationDefinition{}, false
}

// constellationAbbrevToName converts 3-letter abbreviation to full name
func constellationAbbrevToName(abbrev string) string {
	if constell, found := GetConstellationByAbbrev(abbrev); found {
		return constell.Name
	}
	return "Unknown"
}

// House is the house object
type House struct {
	ID        int     `json:"id"`
	Longitude float64 `json:"degree_ut"`
	Hsys      string  `json:"hsys"`
}

// init initialises the ephemeris
func init() {
	C.init()
}

// GetPlanets returns traditional celestial bodies (planets) - now returns CelestialBody
func GetPlanets(yr int, mon int, day int, ut float64) ([]CelestialBody, error) {
	// Use the new calculation system with traditional configuration
	config := GetTraditionalBodiesConfig()
	return CalculateCelestialBodies(yr, mon, day, ut, config)
}

// CalculateCelestialBodies calculates astronomical bodies based on configuration
func CalculateCelestialBodies(yr, mon, day int, ut float64, config EphemerisConfig) ([]CelestialBody, error) {
	m.Lock()
	defer m.Unlock()

	var bodies []CelestialBody

	// Determine which bodies to calculate
	bodyIDs := []int{}
	if config.IncludeTraditional {
		// Traditional planets: Sun, Moon, Mercury-Venus-Mars-Jupiter-Saturn
		bodyIDs = append(bodyIDs, 0, 1, 2, 3, 4, 5, 6)
		// Modern planets: Uranus, Neptune, Pluto
		bodyIDs = append(bodyIDs, 7, 8, 9)
	}
	if config.IncludeNodes {
		// Lunar nodes and apogees
		bodyIDs = append(bodyIDs, 10, 11, 12, 13)
	}
	if config.IncludeCentaurs {
		// Chiron, Pholus
		bodyIDs = append(bodyIDs, 15, 16)
	}
	if config.IncludeAsteroids {
		// Main asteroids: Ceres, Pallas, Juno, Vesta
		bodyIDs = append(bodyIDs, 17, 18, 19, 20)
	}
	// Add custom bodies if specified
	bodyIDs = append(bodyIDs, config.CustomBodies...)

	// Remove duplicates
	bodyIDs = unique(bodyIDs)

	// Calculate each body
	var calculationErrors []string
	for _, bodyID := range bodyIDs {
		bodyDef, found := GetBodyByID(bodyID)
		if !found {
			continue // Skip unknown bodies
		}

		// Calculate position
		body, err := calculateSingleBody(yr, mon, day, ut, bodyDef, config.UseSpeed)
		if err != nil {
			// Log the error but continue with other bodies
			calculationErrors = append(calculationErrors, fmt.Sprintf("%s: %v", bodyDef.Name, err))
			continue
		}

		bodies = append(bodies, body)
	}

	// If no bodies were calculated successfully, return the errors
	if len(bodies) == 0 && len(calculationErrors) > 0 {
		return nil, fmt.Errorf("all body calculations failed: %v", calculationErrors)
	}

	// Return successfully calculated bodies, even if some failed
	// This allows the system to work with partial data
	return bodies, nil
}

// calculateSingleBody calculates a single celestial body
func calculateSingleBody(yr, mon, day int, ut float64, bodyDef BodyDefinition, useSpeed bool) (CelestialBody, error) {
	var xx [6]C.double
	var flags int32 = SEFLG_SWIEPH

	if useSpeed {
		flags |= SEFLG_SPEED
	}

	res := C.sweCalcUt(
		C.int32(yr),
		C.int32(mon),
		C.int32(day),
		C.double(ut),
		C.int(bodyDef.SEConstant),
		&xx[0],
	)

	if res == -1 {
		return CelestialBody{}, fmt.Errorf("ephemeris calculation failed for %s", bodyDef.Name)
	}

	// Get body name
	name := bodyDef.Name
	if bodyDef.SEConstant < 10 { // For planets 0-9, get name from Swiss Ephemeris
		nameBuf := C.malloc(C.sizeof_char * 256)
		defer C.free(nameBuf)
		C.swe_get_planet_name(C.int(bodyDef.SEConstant), (*C.char)(nameBuf))
		name = C.GoString((*C.char)(nameBuf))
	}

	body := CelestialBody{
		ID:        bodyDef.ID,
		Name:      name,
		Type:      bodyDef.Type,
		Longitude: float64(xx[0]),
		Latitude:  float64(xx[1]),
		Distance:  float64(xx[2]),
		Category:  bodyDef.Category,
		Sequence:  bodyDef.Sequence,
	}

	// Add speed data if requested
	if useSpeed {
		body.SpeedLongitude = float64(xx[3])
		body.SpeedLatitude = float64(xx[4])
		body.SpeedDistance = float64(xx[5])
		body.Retrograde = body.SpeedLongitude < 0
	}

	return body, nil
}

// unique removes duplicate integers from slice
func unique(intSlice []int) []int {
	keys := make(map[int]bool)
	var list []int
	for _, entry := range intSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

// CalculateFixedStars calculates fixed stars and groups them by constellation
func CalculateFixedStars(yr, mon, day int, ut float64, constellations []string, maxMagnitude float64) ([]Constellation, error) {
	m.Lock()
	defer m.Unlock()

	// Read fixed star data from sefstars.txt
	stars, err := readFixedStarsData()
	if err != nil {
		return nil, fmt.Errorf("failed to read fixed stars data: %w", err)
	}

	// Filter stars by magnitude if specified
	if maxMagnitude > 0 {
		filteredStars := []FixedStarData{}
		for _, star := range stars {
			if star.Magnitude <= maxMagnitude {
				filteredStars = append(filteredStars, star)
			}
		}
		stars = filteredStars
	}

	// Group stars by constellation
	constellationMap := make(map[string][]CelestialBody)

	for _, starData := range stars {
		// Calculate star position
		star, err := calculateFixedStar(starData, yr, mon, day, ut)
		if err != nil {
			continue // Skip failed calculations
		}

		// Determine constellation
		constellation := determineConstellation(star.Longitude, star.Latitude)
		if constellation != "" {
			// Check if this constellation is requested
			if len(constellations) > 0 {
				requested := false
				for _, reqConst := range constellations {
					if constellation == reqConst {
						requested = true
						break
					}
				}
				if !requested {
					continue
				}
			}

			constellationMap[constellation] = append(constellationMap[constellation], star)
		}
	}

	// Convert to Constellation structs
	var result []Constellation
	for name, stars := range constellationMap {
		constDef, found := GetConstellationByAbbrev(name)
		if !found {
			continue
		}

		result = append(result, Constellation{
			Name:      constDef.Name,
			Abbrev:    constDef.Abbrev,
			LatinName: constDef.LatinName,
			Stars:     stars,
			StarCount: len(stars),
		})
	}

	return result, nil
}

// FixedStarData represents a fixed star from the data file
type FixedStarData struct {
	Name          string
	Abbrev        string
	RA            float64 // Right Ascension in degrees
	Dec           float64 // Declination in degrees
	Magnitude     float64
	Constellation string
}

// calculateFixedStar calculates position of a single fixed star
func calculateFixedStar(starData FixedStarData, yr, mon, day int, ut float64) (CelestialBody, error) {
	var xx [6]C.double
	serr := make([]C.char, 256)

	// Use star name for calculation (must match sefstars.txt format)
	starName := C.CString(starData.Name)
	defer C.free(unsafe.Pointer(starName))

	// Calculate Julian Day
	jd := C.swe_julday(C.int32(yr), C.int32(mon), C.int32(day), C.double(ut), C.int32(SE_GREG_CAL))

	res := C.swe_fixstar_ut(
		starName,
		C.double(jd),
		C.int32(SEFLG_SWIEPH|SEFLG_SPEED),
		&xx[0],
		&serr[0],
	)

	if res == -1 {
		return CelestialBody{}, fmt.Errorf("fixed star calculation failed for %s: %s", starData.Name, C.GoString(&serr[0]))
	}

	return CelestialBody{
		ID:             -1, // Fixed stars use negative IDs
		Name:           starData.Name,
		Type:           TypeFixedStar,
		Constellation:  &starData.Constellation,
		Longitude:      float64(xx[0]),
		Latitude:       float64(xx[1]),
		Distance:       float64(xx[2]),
		SpeedLongitude: float64(xx[3]),
		SpeedLatitude:  float64(xx[4]),
		SpeedDistance:  float64(xx[5]),
		Magnitude:      &starData.Magnitude,
		Retrograde:     float64(xx[3]) < 0,
		Category:       "fixed_star",
	}, nil
}

// readFixedStarsData reads and parses the sefstars.txt file
func readFixedStarsData() ([]FixedStarData, error) {
	// Parse the complete sefstars.txt file instead of hardcoded data
	// Try current directory first (for containers), then fallback to development path
	paths := []string{"sefstars.txt", "sweph/src/sefstars.txt"}

	for _, filepath := range paths {
		if _, err := os.Stat(filepath); err == nil {
			return parseSefstarsFile(filepath)
		}
	}

	return nil, fmt.Errorf("sefstars.txt not found in any expected location")
}

// parseSefstarsFile parses the Swiss Ephemeris sefstars.txt file
func parseSefstarsFile(filepath string) ([]FixedStarData, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sefstars.txt: %w", err)
	}
	defer file.Close()

	var stars []FixedStarData
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue // Skip comments and empty lines
		}

		star, err := parseStarLine(line)
		if err != nil {
			// Log parsing errors but continue with other stars
			continue
		}
		stars = append(stars, star)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading sefstars.txt: %w", err)
	}

	return stars, nil
}

// parseStarLine parses a single line from sefstars.txt
func parseStarLine(line string) (FixedStarData, error) {
	parts := strings.Split(line, ",")
	if len(parts) < 13 {
		return FixedStarData{}, fmt.Errorf("invalid star line format: not enough fields")
	}

	// Extract star name (first field, may contain spaces)
	name := strings.TrimSpace(parts[0])

	// Extract abbreviation (second field, like "alTau", "bePer")
	abbrev := strings.TrimSpace(parts[1])
	constellation := extractConstellationFromAbbrev(abbrev)

	// Parse RA coordinates (fields 3, 4, 5: hours, minutes, seconds)
	raHours, err := strconv.Atoi(strings.TrimSpace(parts[3]))
	if err != nil {
		return FixedStarData{}, fmt.Errorf("invalid RA hours: %w", err)
	}
	raMinutes, err := strconv.Atoi(strings.TrimSpace(parts[4]))
	if err != nil {
		return FixedStarData{}, fmt.Errorf("invalid RA minutes: %w", err)
	}
	raSeconds, err := strconv.ParseFloat(strings.TrimSpace(parts[5]), 64)
	if err != nil {
		return FixedStarData{}, fmt.Errorf("invalid RA seconds: %w", err)
	}

	// Convert to decimal degrees (0-360)
	raDegrees := float64(raHours)*15.0 + float64(raMinutes)*0.25 + raSeconds*15.0/3600.0

	// Parse Dec coordinates (fields 6, 7, 8: degrees, minutes, seconds)
	decSign := 1.0
	decDegStr := strings.TrimSpace(parts[6])
	if strings.HasPrefix(decDegStr, "-") {
		decSign = -1.0
		decDegStr = decDegStr[1:]
	} else if strings.HasPrefix(decDegStr, "+") {
		decDegStr = decDegStr[1:]
	}

	decDegrees, err := strconv.Atoi(decDegStr)
	if err != nil {
		return FixedStarData{}, fmt.Errorf("invalid Dec degrees: %w", err)
	}
	decMinutes, err := strconv.Atoi(strings.TrimSpace(parts[7]))
	if err != nil {
		return FixedStarData{}, fmt.Errorf("invalid Dec minutes: %w", err)
	}
	decSeconds, err := strconv.ParseFloat(strings.TrimSpace(parts[8]), 64)
	if err != nil {
		return FixedStarData{}, fmt.Errorf("invalid Dec seconds: %w", err)
	}

	// Convert to decimal degrees (-90 to +90)
	decDegreesFloat := decSign * (float64(decDegrees) + float64(decMinutes)/60.0 + decSeconds/3600.0)

	// Parse magnitude (field 13 in most cases, but we need to find it reliably)
	magnitude := 999.0 // Default for unknown magnitude

	// Look for magnitude in expected positions, preferring reasonable values
	for i := 9; i < len(parts) && i < 16; i++ {
		part := strings.TrimSpace(parts[i])
		if mag, err := strconv.ParseFloat(part, 64); err == nil {
			// Magnitude should be between -10 and +20 for stars
			if mag >= -10.0 && mag <= 20.0 {
				magnitude = mag
				break
			}
		}
	}

	return FixedStarData{
		Name:          name,
		Abbrev:        abbrev,
		RA:            raDegrees,
		Dec:           decDegreesFloat,
		Magnitude:     magnitude,
		Constellation: constellation,
	}, nil
}

// extractConstellationFromAbbrev extracts constellation abbreviation from star abbreviation
// e.g., "alTau" -> "Tau", "bePer" -> "Per"
func extractConstellationFromAbbrev(abbrev string) string {
	if len(abbrev) < 3 {
		return ""
	}

	// Remove the star designation prefix (usually 2-3 characters)
	// and keep the constellation abbreviation (usually 3 characters)
	for i := len(abbrev) - 3; i >= 0; i-- {
		constAbbrev := abbrev[i:]
		if len(constAbbrev) == 3 && isValidConstellationAbbrev(constAbbrev) {
			return constAbbrev
		}
	}

	return ""
}

// isValidConstellationAbbrev checks if a 3-letter string is a valid constellation abbreviation
func isValidConstellationAbbrev(abbrev string) bool {
	validConstellations := []string{
		"And", "Ant", "Aps", "Aqr", "Aql", "Ara", "Ari", "Aur", "Boo", "Cae",
		"Cam", "Cnc", "CVn", "CMa", "CMi", "Cap", "Car", "Cas", "Cen", "Cep",
		"Cet", "Cha", "Cir", "Col", "Com", "CrA", "CrB", "Crv", "Crt", "Cru",
		"Cyg", "Del", "Dor", "Dra", "Equ", "Eri", "For", "Gem", "Gru", "Her",
		"Hor", "Hya", "Hyi", "Ind", "Lac", "Leo", "LMi", "Lep", "Lib", "Lup",
		"Lyn", "Lyr", "Men", "Mic", "Mon", "Mus", "Nor", "Oct", "Oph", "Ori",
		"Pav", "Peg", "Per", "Phe", "Pic", "Psc", "PsA", "Pup", "Pyx", "Ret",
		"Sge", "Sgr", "Sco", "Scl", "Sct", "Ser", "Sex", "Tau", "Tel", "Tri",
		"TrA", "Tuc", "UMa", "UMi", "Vel", "Vir", "Vol", "Vul",
	}

	for _, valid := range validConstellations {
		if abbrev == valid {
			return true
		}
	}
	return false
}

// determineConstellation determines which constellation a position belongs to
func determineConstellation(longitude, latitude float64) string {
	// Convert ecliptic coordinates to equatorial for constellation boundaries
	// This is a simplified implementation - proper conversion would use obliquity of ecliptic
	ra := longitude // Simplified: assume longitude ≈ RA for bright stars near ecliptic

	constellations := GetAvailableConstellations()
	for _, constell := range constellations {
		if ra >= constell.RAStart && ra <= constell.RAEnd &&
			latitude >= constell.DecStart && latitude <= constell.DecEnd {
			return constell.Abbrev
		}
	}
	return ""
}

// AstroTimeRequest standardizes time input for astronomical calculations
type AstroTimeRequest struct {
	Year      int     `json:"year" validate:"required,min=-2000,max=3000"`
	Month     int     `json:"month" validate:"required,min=1,max=12"`
	Day       int     `json:"day" validate:"required,min=1,max=31"`
	UT        float64 `json:"ut" validate:"min=0,max=24"` // Universal Time
	Gregorian bool    `json:"gregorian"`                  // Default: true
}

// CalculationMetadata contains information about the ephemeris calculation
type CalculationMetadata struct {
	CalculationTimeMs int    `json:"calculation_time_ms"`
	BodiesCalculated  int    `json:"bodies_calculated"`
	Cached            bool   `json:"cached"`
	CacheKey          string `json:"cache_key,omitempty"`
}

// EphemerisResult contains all calculated astronomical data
type EphemerisResult struct {
	Bodies         []CelestialBody     `json:"bodies"`
	Houses         []House             `json:"houses,omitempty"`
	Constellations []Constellation     `json:"constellations,omitempty"`
	Metadata       CalculationMetadata `json:"metadata"`
	Timestamp      time.Time           `json:"timestamp"`
}

// GetTraditionalBodiesConfig returns configuration for traditional planets only
func GetTraditionalBodiesConfig() EphemerisConfig {
	return EphemerisConfig{
		IncludeTraditional: true,
		UseSpeed:           true,
		CalculationFlags:   SEFLG_SWIEPH | SEFLG_SPEED,
	}
}

// GetExtendedBodiesConfig returns configuration for extended bodies (centaurs, asteroids, nodes)
func GetExtendedBodiesConfig() EphemerisConfig {
	return EphemerisConfig{
		IncludeNodes:     true,
		IncludeCentaurs:  true,
		IncludeAsteroids: true,
		UseSpeed:         true,
		CalculationFlags: SEFLG_SWIEPH | SEFLG_SPEED,
	}
}

// EphemerisService defines the interface for ephemeris calculations
type EphemerisService interface {
	// Core methods
	GetPlanetsCached(ctx context.Context, yr, mon, day int, ut float64) ([]CelestialBody, error)
	GetHousesCached(ctx context.Context, yr, mon, day int, ut, lat, lng float64) ([]House, error)
	GetChartCached(ctx context.Context, yr, mon, day int, ut, lat, lng float64) ([]CelestialBody, []House, error)

	// New methods for comprehensive astronomical calculations
	CalculateBodies(ctx context.Context, time AstroTimeRequest, config EphemerisConfig) (*EphemerisResult, error)
	GetTraditionalBodies(ctx context.Context, time AstroTimeRequest) ([]CelestialBody, error)
	GetExtendedBodies(ctx context.Context, time AstroTimeRequest, types []CelestialBodyType) ([]CelestialBody, error)
	GetFixedStars(ctx context.Context, time AstroTimeRequest, constellations []string) ([]Constellation, error)
	GetFullChart(ctx context.Context, time AstroTimeRequest, lat, lng float64) (*EphemerisResult, error)

	// Cached versions
	CalculateBodiesCached(ctx context.Context, time AstroTimeRequest, config EphemerisConfig) (*EphemerisResult, error)
}

// GetAllBodiesConfig returns configuration for all available bodies
func GetAllBodiesConfig() EphemerisConfig {
	return EphemerisConfig{
		IncludeTraditional: true,
		IncludeNodes:       true,
		IncludeCentaurs:    true,
		IncludeAsteroids:   true,
		UseSpeed:           true,
		CalculationFlags:   SEFLG_SWIEPH | SEFLG_SPEED,
	}
}

// GetHouses returns the house positions at the given ut_time
func GetHouses(yr int, mon int, day int, ut float64, lat float64, lng float64) []House {
	m.Lock()
	var houses []House
	cusps := [13]float64{}
	ascmc := [10]float64{}
	c2 := (*C.double)(&cusps[0])
	a2 := (*C.double)(&ascmc[0])

	// Calculate Placidus houses (cusps[0] is Ascendant, cusps[1-12] are houses 1-12)
	C.sweHouses(
		C.int32(yr),
		C.int32(mon),
		C.int32(day),
		C.double(ut),
		C.double(lat),
		C.double(lng),
		C.char('P'), c2, a2)

	// Return houses 1-12 (skip cusps[0] which is Ascendant)
	for i := 1; i <= 12; i++ {
		houses = append(houses, House{
			ID:        i,
			Longitude: cusps[i],
			Hsys:      "P",
		})
	}

	m.Unlock()

	return houses
}

// NewContext checks the incoming data type
// and returns a new Context carrying ephemeris data.
func NewContext(ctx context.Context, ephData interface{}) context.Context {
	if fmt.Sprintf("%T", ephData) == "[]eph.CelestialBody" {
		return context.WithValue(ctx, ephPlanetKey, ephData)
	} else if fmt.Sprintf("%T", ephData) == "[]eph.House" {
		return context.WithValue(ctx, ephHouseKey, ephData)
	}

	// otherwise do nothing
	return ctx
}

// FromContext extracts the eph data from ctx, if present.
func FromContext(ctx context.Context) (interface{}, bool) {
	var (
		ephData interface{}
		ok      bool
	)
	ephData, ok = ctx.Value(ephPlanetKey).([]CelestialBody)
	if !ok {
		ephData, ok = ctx.Value(ephHouseKey).([]House)
	}

	return ephData, ok
}
