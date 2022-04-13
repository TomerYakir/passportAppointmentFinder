package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

const (
	ConsoleGreen = "\033[32m"
	ConsoleReset = "\033[0m"

	// Current coordinates
	MaxNearestLocations = 5
	MinSlotsPerDay      = 1
)

type Location struct {
	Name      string `json:"LocationName"`
	Id        int    `json:"LocationId"`
	ServiceId int    `json:"ServiceId"`
}

func getLocations(top int, lat, lng float64) ([]Location, error) {
	type Res struct {
		Success      bool       `json:"Success"`
		ErrorMessage string     `json:"ErrorMessage"`
		Results      []Location `json:"Results"`
	}
	locationQuery := `https://central.qnomy.com/CentralAPI/LocationSearch?currentPage=1&isFavorite=false&orderBy=Distance&organizationId=56&position=%7B%22lat%22:%22` + fmt.Sprintf("%v", lat) +
		`%22,%22lng%22:%22` + fmt.Sprintf("%v", lng) + `%22,%22accuracy%22:1440%7D&resultsInPage=100&serviceTypeId=156&src=mvws`
	resp, err := http.Get(locationQuery)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var res Res
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	if !res.Success {
		return nil, fmt.Errorf("got success=false from getLocations. full res=%v", res)
	}
	if len(res.Results) <= top {
		top = len(res.Results)
	}
	return res.Results[:top], nil
}

func doAuthRequest(url, authToken string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", authToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

type Slots struct {
	Location string `json:"location"`
	Date     string `json:"date"`
	Hour     string `json:"hour"`
}

func getNearestBooking(loc Location, minRes int, authToken string) ([]Slots, error) {
	var slots []Slots
	type Calendar struct {
		CalendarDate string
		CalendarId   int
	}
	type CalendarSlot struct {
		Time int
	}
	type CalRes struct {
		Success      bool       `json:"Success"`
		ErrorMessage string     `json:"ErrorMessage"`
		Results      []Calendar `json:"Results"`
	}
	type SlotRes struct {
		Success      bool           `json:"Success"`
		ErrorMessage string         `json:"ErrorMessage"`
		Results      []CalendarSlot `json:"Results"`
	}
	url := fmt.Sprintf("https://central.qnomy.com/CentralAPI/SearchAvailableDates?maxResults=50&serviceId=%v&startDate=2022-04-10", loc.ServiceId)
	fmt.Printf("searching for %s using serviceId %v\n", loc.Name, loc.ServiceId)
	body, err := doAuthRequest(url, authToken)
	if err != nil {
		return nil, err
	}
	var res CalRes
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	if !res.Success {
		return nil, fmt.Errorf("got success=false from get dates. full res=%v", res)
	}
	if len(res.Results) == 0 {
		fmt.Printf("got 0 results for %s\n", loc.Name)
		return nil, nil
	}
	for _, cal := range res.Results {
		fmt.Printf("checking date %s\n", cal.CalendarDate)
		url := fmt.Sprintf("https://central.qnomy.com/CentralAPI/SearchAvailableSlots?CalendarId=%v&ServiceId=%v", cal.CalendarId, loc.ServiceId)
		body, err := doAuthRequest(url, authToken)
		if err != nil {
			return nil, err
		}
		var slotRes SlotRes
		if err := json.Unmarshal(body, &slotRes); err != nil {
			return nil, err
		}
		if !slotRes.Success {
			return nil, fmt.Errorf("got success=false from get slots. full res=%v", slotRes)
		}
		if len(slotRes.Results) == 0 {
			fmt.Printf("got 0 results\n")
		}
		if len(slotRes.Results) < minRes {
			fmt.Printf("not enough slots for the day. proceeding\n")
			continue
		}
		for _, t := range slotRes.Results {
			slots = append(slots, Slots{loc.Name, cal.CalendarDate, fmt.Sprintf("%d:%d", t.Time/60, t.Time%60)})
			fmt.Printf("%vAvailable slot at %s - date=%s, time=%d:%d%v\n", ConsoleGreen, loc.Name, cal.CalendarDate, t.Time/60, t.Time%60, ConsoleReset)
		}
	}
	return slots, nil
}

func hello(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": "hello"})
}

func getLocationHandler(c *gin.Context) {
	type Input struct {
		MaxNearestLocations int     `json:"maxNearestLocations" binding:"required"`
		Lat                 float64 `json:"lat" binding:"required"`
		Lng                 float64 `json:"lng" binding:"required"`
	}
	var input Input
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{"err": err.Error(), "errType": "bind error"})
		return
	}
	locs, err := getLocations(input.MaxNearestLocations, input.Lat, input.Lng)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": err.Error(), "errType": "API error"})
	}

	c.JSON(http.StatusOK, locs)
}

func getAppointments(c *gin.Context) {
	type Input struct {
		Locations []Location `json:"locations" binding:"required"`
		Auth      string     `json:"jwt" binding:"required"`
	}
	var input Input
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{"err": err.Error(), "errType": "bind error"})
		return
	}
	var allSlots []Slots
	for _, loc := range input.Locations {
		slots, err := getNearestBooking(loc, MinSlotsPerDay, input.Auth)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"err": err.Error(), "errType": "getNearestBooking"})
			return
		}
		allSlots = append(allSlots, slots...)
	}
	c.JSON(http.StatusOK, allSlots)
}

func main() {

	r := gin.Default()
	r.Use(cors.Default())
	r.GET("/hello", hello)
	r.POST("/locations", getLocationHandler)
	r.POST("/appointments", getAppointments)
	if err := r.Run(); err != nil {
		panic(err)
	}

	/*
		locs, err := getLocations(MaxNearestLocations, Lat, Lng)
		if err != nil {
			panic(err)
		}

	*/
}
