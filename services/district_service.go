package services

import (
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

type District struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func DistrictsByState(c *gin.Context, stateID string) ([]District, error) {
	// Service tự đi lấy DB từ context
	db, err := utils.GetCentralDB()

	if err != nil {
		return nil, err
	}

	query := "SELECT id, name FROM districts where state_id = ? ORDER BY name ASC"
	rows, err := db.Query(query, stateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var districts []District
	for rows.Next() {
		var c District
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			continue
		}
		districts = append(districts, c)
	}
	return districts, nil
}

func WardsByDistrict(c *gin.Context, districtID string) ([]District, error) {
	// Service tự đi lấy DB từ context
	db, err := utils.GetCentralDB()

	if err != nil {
		return nil, err
	}

	query := "SELECT id, name FROM wards where district_id = ? ORDER BY name ASC"
	rows, err := db.Query(query, districtID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var wards []District
	for rows.Next() {
		var c District
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			continue
		}
		wards = append(wards, c)
	}
	return wards, nil
}
