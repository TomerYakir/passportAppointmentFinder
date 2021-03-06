package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

const Auth = "JWT eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiIsIng1dCI6InljeDFyWFRmalRjQjZIQWV1aGxWQklZZmZUbyJ9.eyJpc3MiOiJodHRwOi8vY2VudHJhbC5xbm9teS5jb20iLCJhdWQiOiJodHRwOi8vY2VudHJhbC5xbm9teS5jb20iLCJuYmYiOjE2NDk1Nzc3NzcsImV4cCI6MTY4MDY4MTc3NywidW5pcXVlX25hbWUiOiI4NWNhYjBlYS1mZmQ1LTQyN2EtOGY5ZS1mNDRhNzllZTIyMzYifQ.FtFbXIAVavbZgr2XuzSVJWFOwtysKVqh2-DB2GhmAZgfgOCDskxJGtrUY4g5V41Ly-WuA_5ofbGBd2nAxHO8pfP47bk8ec7YEra0FJRpXxJkrNXiFciJlcw3QFJa7h2CSqj_sVi2eEhgJwa5GPLPRZZE-G1wgUzHJlSg3rgmRvpHb75bqPQi6WGXPvusXa_8IvUd0WQdW5X0_HM6SnNkfO6AlBJm-OmYESMi9NPKj-gr-p5MKpY8rC4mjVcpzQ9bjJndcqXbxEKo0n68Wd7BS1yhBEXXchFCMGBNgEVkbm5jRt5D_Kp3xfJRVR7f1oiMkn6RxqQrsA29FKHShxNwPQ"

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
	body, err := doAuthRequest(locationQuery, Auth)
	if err != nil {
		log.Println("failed to do auth request", err)
		return nil, err
	}
	var res Res
	if err := json.Unmarshal(body, &res); err != nil {
		log.Println("failed to unmarshal", string(body), err)
		return nil, err
	}
	if !res.Success {
		log.Println("got success false. full response", string(body))
		return nil, fmt.Errorf("got success=false from getLocations. full res=%v", res)
	}
	if len(res.Results) <= top {
		top = len(res.Results)
	}
	return res.Results[:top], nil
}

func doAuthRequest(url, authToken string) ([]byte, error) {
	log.Println("doing request", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if authToken != "" {
		req.Header.Add("Authorization", authToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("got response code %d. full response: %v", resp.StatusCode, body)
	}
	return body, nil
}

type Slots struct {
	Location string `json:"location"`
	Date     string `json:"date"`
	Hour     string `json:"hour"`
}

func isStrDateAfterDate(d1, d2 string) (bool, error) {
	if d2 == "" {
		return false, nil
	}
	if strings.Contains(d1, "T") {
		d1 = strings.Split(d1, "T")[0]
	}
	d1d, err := time.Parse("2006-01-02", d1)
	if err != nil {
		return false, err
	}
	d2d, err := time.Parse("2006-01-02", d2)
	if err != nil {
		return false, err
	}
	return d1d.After(d2d), nil
}

func getNearestBooking(loc Location, minRes int, startDate, toDate, authToken string) ([]Slots, error) {
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
	url := fmt.Sprintf("https://central.qnomy.com/CentralAPI/SearchAvailableDates?maxResults=50&serviceId=%v&startDate=%s", loc.ServiceId, startDate)
	log.Printf("searching for %s using serviceId %v\n", loc.Name, loc.ServiceId)
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
		log.Printf("got 0 results for %s\n", loc.Name)
		return nil, nil
	}
	for _, cal := range res.Results {
		log.Printf("checking date %s\n", cal.CalendarDate)
		isAfter, err := isStrDateAfterDate(cal.CalendarDate, toDate)
		if err != nil {
			return nil, fmt.Errorf("failed to parse dates. err=%v", err)
		}
		if isAfter {
			log.Printf("date %s is beyond end date=%s\n", cal.CalendarDate, toDate)
			break
		}
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
			log.Printf("got 0 results\n")
		}
		if len(slotRes.Results) < minRes {
			log.Printf("not enough slots for the day. proceeding\n")
			continue
		}
		for _, t := range slotRes.Results {
			slots = append(slots, Slots{loc.Name, cal.CalendarDate, fmt.Sprintf("%d:%d", t.Time/60, t.Time%60)})
			log.Printf("Available slot at %s - date=%s, time=%d:%d\n", loc.Name, cal.CalendarDate, t.Time/60, t.Time%60)
		}
	}
	return slots, nil
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
		StartDate string     `json:"fromDate" binding:"required"`
		EndDate   string     `json:"toDate"`
		MinSlots  int        `json:"minSlots"`
	}
	var input Input
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{"err": err.Error(), "errType": "bind error"})
		return
	}
	var allSlots []Slots
	for _, loc := range input.Locations {
		slots, err := getNearestBooking(loc, input.MinSlots, input.StartDate, input.EndDate, Auth)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"err": err.Error(), "errType": "getNearestBooking"})
			return
		}
		allSlots = append(allSlots, slots...)
	}
	c.JSON(http.StatusOK, allSlots)
}

func main() {

	clientHtml, err := os.ReadFile("client/index.html")
	if err != nil {
		panic(err)
	}
	clientIndex, err := os.ReadFile("client/index.js")
	if err != nil {
		panic(err)
	}
	r := gin.Default()
	r.Use(cors.Default())
	r.Use(gin.Logger())
	r.Static("/static", "static")
	r.GET("/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", clientHtml)
	})
	r.GET("/index.js", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", clientIndex)
	})
	r.POST("/locations", getLocationHandler)
	r.POST("/appointments", getAppointments)
	if err := r.Run(); err != nil {
		panic(err)
	}

}
