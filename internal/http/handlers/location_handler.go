// README: Location HTTP handlers — read-only presence queries backed by the
// Redis GEO index (and, for the "list all drivers" case, Firebase RTDB).
//
// Endpoints:
//
//	GET /api/location/drivers                       — list every online driver
//	GET /api/location/drivers/nearby?lat&lng&radius_km — drivers within radius
//	GET /api/location/passengers/nearby?lat&lng&radius_km — passengers within radius
//
// Auth: all routes require the Auth middleware; no role gating in Phase 1.
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"ark/internal/http/dto"
	"ark/internal/httpx"
	"ark/internal/modules/location"
)

// LocationHandler exposes read-only presence queries over the location service.
type LocationHandler struct {
	svc *location.Service
}

// NewLocationHandler returns a LocationHandler backed by the given Service.
func NewLocationHandler(svc *location.Service) *LocationHandler {
	return &LocationHandler{svc: svc}
}

// ListAllDrivers handles GET /api/location/drivers.
//
// @Summary      List all online drivers
// @Description  Returns every driver currently reporting an online position via the RTDB-backed poller. DistanceKm is always 0 because no query origin is supplied; use GET /api/location/drivers/nearby when you need distances.
// @Tags         Location
// @Produce      json
// @Security     FirebaseAuth
// @Success      200  {array}   dto.NearbyDriverResponse
// @Failure      401  {object}  httpx.ErrorBody
// @Failure      500  {object}  httpx.ErrorBody
// @Router       /api/location/drivers [get]
func (h *LocationHandler) ListAllDrivers(c *gin.Context) {
	drivers, err := h.svc.GetAllDrivers(c.Request.Context())
	if err != nil {
		httpx.RespondError(c, http.StatusInternalServerError, "internal error")
		return
	}
	out := make([]dto.NearbyDriverResponse, len(drivers))
	for i, d := range drivers {
		out[i] = dto.NearbyDriverResponse{
			DriverID:   string(d.DriverID),
			Lat:        d.Lat,
			Lng:        d.Lng,
			DistanceKm: d.Distance,
		}
	}
	writeJSON(c, http.StatusOK, out)
}

// ListNearbyDrivers handles GET /api/location/drivers/nearby?lat=&lng=&radius_km=.
//
// @Summary      List nearby drivers
// @Description  Returns online drivers within `radius_km` of (`lat`, `lng`), sorted by distance ascending. Presence is sourced from the Redis GEO index populated by the RTDB poller; drivers whose status TTL has expired are filtered out.
// @Tags         Location
// @Produce      json
// @Security     FirebaseAuth
// @Param        lat        query     number  true  "Query origin latitude"
// @Param        lng        query     number  true  "Query origin longitude"
// @Param        radius_km  query     number  true  "Search radius in kilometres (must be > 0)"
// @Success      200  {array}   dto.NearbyDriverResponse
// @Failure      400  {object}  httpx.ErrorBody
// @Failure      401  {object}  httpx.ErrorBody
// @Failure      500  {object}  httpx.ErrorBody
// @Router       /api/location/drivers/nearby [get]
func (h *LocationHandler) ListNearbyDrivers(c *gin.Context) {
	lat, lng, radius, ok := parseGeoQuery(c)
	if !ok {
		return
	}
	drivers, err := h.svc.GetNearbyDrivers(c.Request.Context(), lat, lng, radius)
	if err != nil {
		httpx.RespondError(c, http.StatusInternalServerError, "internal error")
		return
	}
	out := make([]dto.NearbyDriverResponse, len(drivers))
	for i, d := range drivers {
		out[i] = dto.NearbyDriverResponse{
			DriverID:   string(d.DriverID),
			Lat:        d.Lat,
			Lng:        d.Lng,
			DistanceKm: d.Distance,
		}
	}
	writeJSON(c, http.StatusOK, out)
}

// ListNearbyPassengers handles GET /api/location/passengers/nearby?lat=&lng=&radius_km=.
//
// @Summary      List nearby passengers
// @Description  Returns passengers currently looking for a ride within `radius_km` of (`lat`, `lng`), sorted by distance ascending. Sourced from the Redis GEO index populated by the RTDB poller.
// @Tags         Location
// @Produce      json
// @Security     FirebaseAuth
// @Param        lat        query     number  true  "Query origin latitude"
// @Param        lng        query     number  true  "Query origin longitude"
// @Param        radius_km  query     number  true  "Search radius in kilometres (must be > 0)"
// @Success      200  {array}   dto.NearbyPassengerResponse
// @Failure      400  {object}  httpx.ErrorBody
// @Failure      401  {object}  httpx.ErrorBody
// @Failure      500  {object}  httpx.ErrorBody
// @Router       /api/location/passengers/nearby [get]
func (h *LocationHandler) ListNearbyPassengers(c *gin.Context) {
	lat, lng, radius, ok := parseGeoQuery(c)
	if !ok {
		return
	}
	passengers, err := h.svc.GetNearbyPassengers(c.Request.Context(), lat, lng, radius)
	if err != nil {
		httpx.RespondError(c, http.StatusInternalServerError, "internal error")
		return
	}
	out := make([]dto.NearbyPassengerResponse, len(passengers))
	for i, p := range passengers {
		out[i] = dto.NearbyPassengerResponse{
			PassengerID: string(p.PassengerID),
			Lat:         p.Lat,
			Lng:         p.Lng,
			DistanceKm:  p.Distance,
			Status:      p.Status,
		}
	}
	writeJSON(c, http.StatusOK, out)
}

// parseGeoQuery reads lat, lng, and radius_km from the query string and writes
// a 400 response directly when anything is missing or invalid. The ok return
// signals whether the caller may proceed with the parsed values.
func parseGeoQuery(c *gin.Context) (lat, lng, radiusKm float64, ok bool) {
	latStr := c.Query("lat")
	lngStr := c.Query("lng")
	radiusStr := c.Query("radius_km")
	if latStr == "" || lngStr == "" || radiusStr == "" {
		httpx.RespondError(c, http.StatusBadRequest, "lat, lng, and radius_km are required")
		return 0, 0, 0, false
	}
	var err error
	if lat, err = strconv.ParseFloat(latStr, 64); err != nil {
		httpx.RespondError(c, http.StatusBadRequest, "invalid lat")
		return 0, 0, 0, false
	}
	if lng, err = strconv.ParseFloat(lngStr, 64); err != nil {
		httpx.RespondError(c, http.StatusBadRequest, "invalid lng")
		return 0, 0, 0, false
	}
	if radiusKm, err = strconv.ParseFloat(radiusStr, 64); err != nil {
		httpx.RespondError(c, http.StatusBadRequest, "invalid radius_km")
		return 0, 0, 0, false
	}
	if radiusKm <= 0 {
		httpx.RespondError(c, http.StatusBadRequest, "radius_km must be positive")
		return 0, 0, 0, false
	}
	return lat, lng, radiusKm, true
}
