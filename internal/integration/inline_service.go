package integration

import (
	"fmt"
	"strings"
)

// BookInline simulates a reservation via the Inline booking platform.
// It returns a formatted Traditional Chinese confirmation message.
// In production this would call the real Inline API.
func BookInline(restaurantName, dateTime, userName, userPhone string, passengerCount int) string {
	// Format phone number as 0912-xxx-xxx for display (mask middle digits).
	maskedPhone := maskPhone(userPhone)

	personWord := "位"
	if passengerCount <= 0 {
		passengerCount = 1
	}

	return fmt.Sprintf(
		"已透過 Inline 成功為您保留位子！\n餐廳：%s\n時間：%s\n人數：%d %s\n訂位人：%s（%s）\n\n請於用餐時間準時到場，祝您用餐愉快！",
		restaurantName,
		dateTime,
		passengerCount,
		personWord,
		userName,
		maskedPhone,
	)
}

// maskPhone partially masks a phone number for privacy display.
// e.g. "0912345678" → "0912-xxx-678"
func maskPhone(phone string) string {
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.ReplaceAll(phone, " ", "")
	if len(phone) == 10 {
		return phone[:4] + "-xxx-" + phone[7:]
	}
	// Fallback: just return as-is if unexpected format
	return phone
}
