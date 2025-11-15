package eph

/*
#cgo CFLAGS: -I${SRCDIR}/sweph/src
#cgo LDFLAGS: -L${SRCDIR}/sweph/src -lswe -ldl -lm
#include "swephexp.h"

int init()
{
	swe_set_ephe_path(NULL);
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
	"context"
	"fmt"
	"sync"
)

type key int

const ephPlanetKey key = 0
const ephHouseKey key = 1

// m ensures exclusive ephemeris interaction, preventing data racing.
var m sync.Mutex

// Planet is the planet object
type Planet struct {
	ID         int     `json:"id"`
	Name       string  `json:"name"`
	Longitude  float64 `json:"degree_ut"`
	Retrograde bool    `json:"retrograde"`
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

// GetPlanets returns the planetary positions at the given ut_time
func GetPlanets(yr int, mon int, day int, ut float64) ([]Planet, error) {
	m.Lock()
	var p Planet
	var planets []Planet
	const nP = 12
	xx := [nP][6]float64{}

	for i := 0; i < nP; i++ {
		x2 := (*C.double)(&xx[i][0])
		var pName *C.char = C.CString(*new(string))

		res := C.sweCalcUt(
			C.int32(yr),
			C.int32(mon),
			C.int32(day),
			C.double(ut), C.int32(i), x2)

		if res == -1 {
			return planets, fmt.Errorf("ephemeris read error")
		}

		C.swe_get_planet_name(C.int32(i), pName)

		p.ID = i
		p.Name = string(C.GoString(pName))
		p.Longitude = xx[i][0]
		p.Retrograde = xx[i][3] < 0
		planets = append(planets, p)
	}

	m.Unlock()

	return planets, nil
}

// GetHouses returns the house positions at the given ut_time
func GetHouses(yr int, mon int, day int, ut float64, lat float64, lng float64) []House {
	m.Lock()
	var h House
	var houses []House
	cusps := [13]float64{}
	ascmc := [10]float64{}
	c2 := (*C.double)(&cusps[0])
	a2 := (*C.double)(&ascmc[0])

	calcHouses := func(hsys rune) {
		C.sweHouses(
			C.int32(yr),
			C.int32(mon),
			C.int32(day),
			C.double(ut),
			C.double(lat),
			C.double(lng),
			C.char(hsys), c2, a2)

		for i, _h := range cusps {
			h.ID = i
			h.Longitude = _h
			h.Hsys = string(hsys)
			houses = append(houses, h)
		}
	}

	calcHouses('P')
	calcHouses('K')
	calcHouses('E')
	calcHouses('W')

	m.Unlock()

	return houses
}

// NewContext checks the incoming data type
// and returns a new Context carrying ephemeris data.
func NewContext(ctx context.Context, ephData interface{}) context.Context {
	if fmt.Sprintf("%T", ephData) == "[]eph.Planet" {
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
	ephData, ok = ctx.Value(ephPlanetKey).([]Planet)
	if !ok {
		ephData, ok = ctx.Value(ephHouseKey).([]House)
	}

	return ephData, ok
}
