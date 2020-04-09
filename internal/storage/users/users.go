package users

import (
	"fmt"

	"github.com/alexwbaule/give-help/v2/generated/models"
	"github.com/alexwbaule/give-help/v2/internal/storage"
	"github.com/lib/pq"
	"github.com/prometheus/common/log"
)

//Users Object struct
type Users struct {
	conn *storage.Connection
}

//New creates a new instance
func New(conn *storage.Connection) *Users {
	return &Users{conn: conn}
}

const upsertUser string = `
INSERT INTO USERS 
(
	UserID,
	Name,
	Description,
	DeviceID,
	AllowShareData,
	Tags,
	Images,

	--Reputation
	Giver,
	Taker,

	--Contact
	URL,
	Email,
	Facebook,
	Instagram,
	Google,
	AdditionalData,

	--Contact Address
	Address,
	City,
	State,
	ZipCode,
	Country,

	--point
	Lat,
	Long
)
VALUES
(
	$1,
	$2,
	$3,
	$4,
	$5,
	$6,
	$7,

	--Reputation
	$8,
	$9,

	--Contact
	$10,
	$11,
	$12,
	$13,
	$14,
	$15,

	--Contact Address
	$16,
	$17,
	$18,
	$19,
	$20,	
	
	--point
	$21,
	$22	
)
ON CONFLICT (UserID) 
DO
	UPDATE
	SET 
		Name = $2,
		Description = $3,
		DeviceID = $4,
		AllowShareData = $5,
		Tags = $6,
		Images = $7,
		
		--Reputation
		Giver = $8,
		Taker = $9,

		--Contact
		URL = $10,
		Email = $11,
		Facebook = $12,
		Instagram = $13,
		Google = $14,
		AdditionalData = $15,
		
		--Contact Address
		Address = $16,
		City = $17,
		State = $18,
		ZipCode = $19,
		Country = $20,

		--point
		Lat = $21,
		Long = $22;
`

//Upsert insert or update on database
func (u *Users) Upsert(user *models.User) error {
	if user == nil {
		return fmt.Errorf("cannot insert an empty user struct")
	}

	if len(user.UserID) == 0 {
		return fmt.Errorf("cannot insert an empty userID")
	}

	repGiver := 0.0
	repTaker := 0.0

	if user.Reputation != nil {
		repGiver = user.Reputation.Giver
		repTaker = user.Reputation.Taker
	}

	url := ""
	email := ""
	facebook := ""
	instagram := ""
	google := ""
	additionalData := ""

	if user.Contact != nil {
		url = user.Contact.URL
		email = user.Contact.Email
		facebook = user.Contact.Facebook
		instagram = user.Contact.Instagram
		google = user.Contact.Google
		additionalData = user.Contact.AdditionalData
	}

	address := ""
	city := ""
	state := ""
	zipCode := int64(0)
	country := ""

	lat := 0.0
	long := 0.0

	if user.Location != nil {
		address = user.Location.Address
		city = user.Location.City
		state = user.Location.State
		zipCode = user.Location.ZipCode
		country = user.Location.Country

		lat = user.Location.Lat
		long = user.Location.Long
	}

	db := u.conn.Get()
	defer db.Close()

	_, err := db.Exec(
		upsertUser,
		user.UserID,
		user.Name,
		user.Description,
		user.DeviceID,
		user.AllowShareData,
		pq.Array(user.Tags),
		pq.Array(user.Images),
		repGiver,
		repTaker,
		url,
		email,
		facebook,
		instagram,
		google,
		additionalData,
		address,
		city,
		state,
		zipCode,
		country,
		lat,
		long,
	)

	if err != nil {
		log.Errorf("fail to try insert or update user on database: error = %s", err.Error())
		return u.conn.CheckError(err)
	}

	err = u.insertPhones(user)

	return u.conn.CheckError(err)
}

const selectUser string = `
SELECT 
	Name,
	Description,
	DeviceID,
	AllowShareData,
	Tags,
	Images,

	--Reputation
	Giver,
	Taker,

	--Contatct
	URL,
	Email,
	Facebook,
	Instagram,
	Google,
	AdditionalData,

	--Contact Address
	Address,
	City,
	State,
	ZipCode,
	Country,

	--point
	Lat,
	Long
FROM
	USERS
WHERE
	UserID = $1;
`

//Load load from database
func (u *Users) Load(userID string) (*models.User, error) {
	user := models.User{
		UserID:     models.ID(userID),
		Contact:    &models.Contact{},
		Reputation: &models.Reputation{},
		Location:   &models.Location{},
	}

	db := u.conn.Get()
	defer db.Close()

	row := db.QueryRow(selectUser, userID)

	var tags []string

	err := row.Scan(
		&user.Name,
		&user.Description,
		&user.DeviceID,
		&user.AllowShareData,
		pq.Array(&tags),
		pq.Array(&user.Images),
		&user.Reputation.Giver,
		&user.Reputation.Taker,
		&user.Contact.URL,
		&user.Contact.Email,
		&user.Contact.Facebook,
		&user.Contact.Instagram,
		&user.Contact.Google,
		&user.Contact.AdditionalData,
		&user.Location.Address,
		&user.Location.City,
		&user.Location.State,
		&user.Location.ZipCode,
		&user.Location.Country,
		&user.Location.Lat,
		&user.Location.Long,
	)

	user.Tags = models.Tags(tags)

	if err != nil {
		log.Errorf("fail to try load user from database: error = %s", err.Error())
		return &user, u.conn.CheckError(err)
	}

	user.Contact.Phones, err = u.loadPhones(userID)

	return &user, u.conn.CheckError(err)
}

const insertPhones string = `
INSERT INTO
PHONES
(
	UserID,
	CountryCode,
	IsDefault,
	PhoneNumber,
	Region,
	WhatsApp
)
VALUES
	(
		$1,
		$2,
		$3,
		$4,
		$5,
		$6
	);
`

const cleanPhones string = `
DELETE FROM 
	PHONES
WHERE
	UserID = $1
`

func (u *Users) insertPhones(user *models.User) error {
	var err error

	if user != nil {
		if user.Contact != nil {
			if len(user.Contact.Phones) > 0 {
				db := u.conn.Get()
				defer db.Close()

				_, err = db.Exec(cleanPhones, user.UserID)

				if err != nil {
					return u.conn.CheckError(err)
				}

				for _, p := range user.Contact.Phones {
					_, err := db.Exec(
						insertPhones,
						user.UserID,
						p.CountryCode,
						p.IsDefault,
						p.PhoneNumber,
						p.Region,
						p.Whatsapp,
					)

					if err != nil {
						return u.conn.CheckError(err)
					}
				}
			}
		}
	}

	return u.conn.CheckError(err)
}

const selectPhones = `
SELECT 
	CountryCode,
	IsDefault,
	PhoneNumber,
	Region,
	WhatsApp
FROM
	PHONES
WHERE
	UserID = $1
ORDER BY
	IsDefault DESC,
	CreatedAt
;
`

func (u *Users) loadPhones(userID string) ([]*models.Phone, error) {
	ret := []*models.Phone{}

	db := u.conn.Get()
	defer db.Close()

	rows, err := db.Query(selectPhones, userID)

	if err != nil {
		return ret, u.conn.CheckError(err)
	}

	defer rows.Close()

	for rows.Next() {
		p := models.Phone{}

		err = rows.Scan(
			&p.CountryCode,
			&p.IsDefault,
			&p.PhoneNumber,
			&p.Region,
			&p.Whatsapp,
		)

		if err != nil {
			return ret, u.conn.CheckError(err)
		}

		ret = append(ret, &p)
	}

	return ret, u.conn.CheckError(err)
}