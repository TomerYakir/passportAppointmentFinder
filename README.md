# passportAppointmentFinder
 
Automatically searches an available appointment in the Ministry of the Interior Office
Port of https://github.com/frankbolton/IsraelPassportBooking (jupyter/python-based) to Golang with some additional options (Thanks Frank!)

# How to use?
1. Login to the myvisit website and extract the JWT auth (using the network console)
2. Change the following constants:
- Lat - your location latitude
- Lng - your location longitue
- MaxNearestLocations - maximum nearest locations to use for the search
- MinSlotsPerDay - minimum slots per day (in case you need more than 1 appointment, otherwise set to 1)	
- JWT - auth token - should look like `JWT asdlkfasdkfjdslkfadjsfkl...`
3. Download go1.17+
4. Run `go run main.go`