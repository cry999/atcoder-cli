package adt

import (
	"fmt"
	"path/filepath"
	"time"
)

// dailyHolds is dailyHolds[weekday][time] = number in the day
var dailyHolds = map[time.Weekday]map[string]int{
	time.Tuesday:   {"1530": 1, "1730": 2, "1930": 3},
	time.Wednesday: {"1600": 1, "1800": 2, "2000": 3},
	time.Thursday:  {"1630": 1, "1830": 2, "2030": 3},
}
var dailyTimes = map[time.Weekday]map[int]string{
	time.Tuesday:   {1: "1530", 2: "1730", 3: "1930"},
	time.Wednesday: {1: "1600", 2: "1800", 3: "2000"},
	time.Thursday:  {1: "1630", 2: "1830", 3: "2030"},
}

type Family struct {
	date   time.Time
	number int
	level  Level
}

func New(rawDate, rawTime, rawLevel string) (*Family, error) {
	date, err := time.Parse("20060102", rawDate)
	if err != nil {
		return nil, err
	}
	timeToNumber, ok := dailyHolds[date.Weekday()]
	if !ok {
		return nil, err
	}
	number, ok := timeToNumber[rawTime]
	if !ok {
		return nil, err
	}

	// TODO: validate rawLevel
	return &Family{date: date, number: number, level: Level(rawLevel)}, nil
}

func (f *Family) ContestName() string {
	return fmt.Sprintf("adt_%s_%s_%d", f.level, f.date.Format("20060102"), f.number)
}

func (f *Family) BaseDir(workdir string) string {
	return filepath.Join(
		workdir,
		"adt",
		fmt.Sprintf("%04d", f.date.Year()),
		fmt.Sprintf("%02d", f.date.Month()),
		fmt.Sprintf("%02d", f.date.Day()),
		dailyTimes[f.date.Weekday()][f.number],
	)
}
