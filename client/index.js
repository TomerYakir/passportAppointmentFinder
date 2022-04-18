updateStatus = function(status) {
    $("#status").text(status);
}

findAppointments = async function() {
    updateStatus("מחפש...");
    let lat = parseFloat($("#lat").val());
    let lng = parseFloat($("#lng").val());
    let fromDate = $("#fromDate").val();
    let toDate = $("#toDate").val();
    let minSlots = parseInt($("#minSlots").val());
    let maxNearestLocations = parseInt($("#maxNearestLocations").val());
    let locations;
    try {
        locations = await axios.post('/locations', {
            maxNearestLocations,
            lat,
            lng
        });
    } catch (err) {
        updateStatus(`שגיאה - ${err.message}`);
        return;
    }
    
    const locNames = locations.data.map(l => l.LocationName);
    let foundCount=0;
    updateStatus(`מחפש בלשכות הבאות: ${locNames.join(",")}`);
    for (let i=0; i<locations.data.length; i++) {
        updateStatus(`מחפש ב: ${locNames[i]}`);
        try {
            let data = await axios.post('/appointments', {
                "locations": [ locations.data[i] ],
                "fromDate": fromDate,
                "toDate": toDate,
                "minSlots": minSlots,
            });
            if (!data.data || data.data.length == 0) {
                // nothing found here
            } else {
                foundCount += data.data.length;
                addSlotsToTable(data.data);
            }
        } catch (err) {
            console.error(`שגיאה - ${err.message}`)
        }
    }
    if (!foundCount) {
        updateStatus("לא נמצאו תורים. נסו לשנות פרמטרים")
    } else {
        updateStatus("החיפוש הסתיים")
    }
    
}
 
function getPosition(options) {
    return new Promise((resolve, reject) => 
        navigator.geolocation.getCurrentPosition(resolve, reject, options)
    );
}

async function getCurrentLocation() {
    var geoOptions = {
        enableHighAccuracy: true,
        timeout: 5000,
        maximumAge: 0
    };
    let data;
    const pos = await getPosition(geoOptions);
    const mapUrl = `https://www.mapquestapi.com/geocoding/v1/reverse?key=IkcEuF1QqyNeGJiTzzMSMtztCFG4A93V&location=${pos.coords.latitude}%2C${pos.coords.longitude}&outFormat=json&thumbMaps=false`
    try {
        data = await axios.get(mapUrl);
        data = data.data;
        if (data && data.results && data.results.length > 0 && data.results[0].locations && data.results[0].locations.length > 0) {
            updateStatus("לחץ על חפש כדי לחפש תורים פנויים")
            const loc = data.results[0].locations[0];
            return {"city": loc.adminArea5, "street": loc.street, "lat": pos.coords.latitude, "lng": pos.coords.longitude};
        };
    } catch (err) {
        updateStatus(`שגיאה - ${err.message}`)
    }
    
}

function prettyDate(date) {
    return date.split("T")[0];
}

function formatSlot(slot) {
    const slotParts = slot.split(":")
    return slotParts[0] + ":" + ("0"+ slotParts[1]).slice(-2)
}

function addDateToTable(loc, date, slots) {
    $("#slots").append(`<tr><td>${loc}</td><td>${prettyDate(date)}</td><td>${slots.map(s => formatSlot(s)).join(" ")}</td></tr>`);
}

function addSlotsToTable(data) {
    if (!data) {
        return;
    }
    let mapped = {};
    data.forEach((o) => {
        let key = JSON.stringify({"location": o.location, "date": o.date});
        if (!mapped[key]) {
            mapped[key] = [ `${o.hour}` ];
        } else {
            mapped[key].push(`${o.hour}`);
        }
    });
    Object.keys(mapped).forEach((k) => {
        const key = JSON.parse(k);
        const val = mapped[k];
        addDateToTable(key.location, key.date, val);
    });
}