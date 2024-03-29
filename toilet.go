package main

import (
	"context"
	"errors"
	"time"

	"cloud.google.com/go/datastore"
)

// Toilet is the container that stores all dumps.
type Toilet struct {
	ID            string    // The Name of the bucket
	Created       time.Time // When it was created
	ResponseCode  int       // The code of the response to return
	ResponseBody  string    // The body of the response to return when posted to
	ResponseDelay int       // How long to sleep before sending the response to the client
	AuthUsername  string    // A username for toilets with HTTP Auth
	AuthPassword  string    // A password for toilets with HTTP Auth
	LastDelete    time.Time // The last time dumps were deleted from this toilet

	// If true, the post body will be dumped before the params
	// this breaks url encoded forms but allows people to see the raw version of their forms
	DumpBodyFirst bool
}

// retrieves a toilet from the data store
func getToilet(context context.Context, client *datastore.Client, toiletID string) (*Toilet, error) {
	var toilet Toilet
	toiletKey := datastore.NameKey("Toilet", toiletID, nil)
	err := client.Get(context, toiletKey, &toilet)
	if err != nil {
		if err == datastore.ErrNoSuchEntity {
			logMessage(context, "Unable to find toilet: "+toiletID)
			return nil, nil
		}

		logError(context, "Error looking up toilet: "+toiletID, err)
		return nil, err
	}

	return &toilet, nil
}

// Finds all dumps for a given toilet
func getToiletDumps(context context.Context, client *datastore.Client, toiletID string) ([]Dump, error) {
	var dumps []Dump
	toiletKey := datastore.NameKey("Toilet", toiletID, nil)
	q := datastore.NewQuery("Dump").Ancestor(toiletKey).Order("Timestamp")

	// Get the dumps
	if _, err := client.GetAll(context, q, &dumps); err != nil {
		logError(context, "Failed querying for dumps.", err)
		return nil, err
	}

	return dumps, nil
}

// Gets the ID of the latest dump in a toilet
func getLatestDumpFromToilet(context context.Context, client *datastore.Client, toiletID string) (int64, error) {
	toiletKey := datastore.NameKey("Toilet", toiletID, nil)
	q := datastore.NewQuery("Dump").Ancestor(toiletKey).Order("-Timestamp").KeysOnly().Limit(1)

	keys, err := client.GetAll(context, q, nil)
	if err != nil {
		logError(context, "Failed getting latest dump.", err)
		return -1, err
	}
	if len(keys) < 1 {
		return -1, errors.New("this pristine toilet has no dumps")
	}

	return keys[0].ID, nil
}

// Deletes all dumps for a given toilet
func flushAllDumps(context context.Context, client *datastore.Client, toiletID string) error {
	toiletKey := datastore.NameKey("Toilet", toiletID, nil)
	toDelete := datastore.NewQuery("Dump").Ancestor(toiletKey).KeysOnly()

	keys, err := client.GetAll(context, toDelete, nil)
	if err != nil {
		logError(context, "FlushAllDumps: Error retrieving keys to delete.", err)
		return err
	}

	err = client.DeleteMulti(context, keys)
	if err != nil {
		logError(context, "FlushAllDumps: Unable to delete items", err)
		return err
	}

	return nil
}

// Gets all Disabled Toilets
func getDisabledToilets(context context.Context, client *datastore.Client) ([]Toilet, error) {
	var toilets []Toilet

	q := datastore.NewQuery("Toilet").Filter("ResponseCode =", -1)
	if _, err := client.GetAll(context, q, &toilets); err != nil {
		logError(context, "Failed getting blocked toilets", err)
		return nil, err
	}

	return toilets, nil
}

// Toilets mustn't have more than MaxDumpsIntoilet. So do a check and delete any if we are over
func deleteExtraDumps(context context.Context, client *datastore.Client, toilet *Toilet) error {

	toiletKey := datastore.NameKey("Toilet", toilet.ID, nil)
	count, err := client.Count(context, datastore.NewQuery("Dump").Ancestor(toiletKey))
	if err != nil {
		logError(context, "Error getting count of dumps for this toilet.", err)
		return err
	}

	// If the toilet has more dumps in it then it should
	if count >= MaxDumpsInToilet {
		// If the last time dumps were cleared is too recent then this is a spammy toilet
		// and it must get shut off
		if time.Since(toilet.LastDelete).Seconds() < MinSecondsBetweenDeletes {
			toilet.ResponseCode = -1
			if _, err := updateToilet(context, client, toilet); err != nil {
				logError(context, "Error storing dump", err)
				return err
			}
			return errors.New("Too many dumps. Toilet clogged")
		}

		// If there are more dumps in the toilet than the current limit, clear some space
		toDelete := datastore.NewQuery("Dump").Ancestor(toiletKey).Order("-Timestamp").Limit(NumDumpsToDelete).KeysOnly()
		keys, err := client.GetAll(context,toDelete, nil)
		if err != nil {
			logError(context, "Error retrieving keys to delete.", err)
			return err
		}
		err = client.DeleteMulti(context, keys)
		if err != nil {
			logError(context, "Unable to delete items", err)
			return err
		}

		// Make note of when we cleared space.
		toilet.LastDelete = time.Now()
		if _, err := updateToilet(context, client, toilet); err != nil {
			logError(context, "Error storing dump", err)
			return err
		}
	}

	return nil
}

// Stores a toilet
func storeToilet(context context.Context, client *datastore.Client, toilet *Toilet, toiletID string) (string, error) {
	key := datastore.NameKey("Toilet", toiletID, nil)
	if _, err := client.Put(context, key, toilet); err != nil {
		logError(context, "Unable to store toilet", err)
		return "", err
	}

	return toiletID, nil
}

// Updates a toilet
//func updateToilet(context context.Context, toilet *Toilet, toiletID string) (string, error) {
func updateToilet(context context.Context, client *datastore.Client, toilet *Toilet) (string, error) {
	toiletKey := datastore.NameKey("Toilet", toilet.ID, nil)
	if _, err := client.Put(context, toiletKey, toilet); err != nil {
		logError(context, "Unable to store toilet", err)
		return "", err
	}

	return toilet.ID, nil
}

// Creates a new toilet
func createToilet(context context.Context, client *datastore.Client, toiletID string) (*Toilet, error) {
	if toiletID == "" {
		return nil, errors.New("Can't create a toilet if name is empty")
	}

	if !isValidID(toiletID) {
		return nil, errors.New("Toilet name is invalid")
	}

	// This redundant lookup is neccessary to keep from squishing an existing toilet
	toilet, err := getToilet(context,client, toiletID)
	if err != nil {
		logError(context, "Errr on duplicate check for toilet: "+toiletID, err)
		return nil, err
	}
	if toilet != nil {
		return nil, errors.New("Can't create a toilet which already exists")
	}

	// Create the new toilet
	toilet = new(Toilet)
	toilet.Created = time.Now()
	toilet.ID = toiletID
	toilet.ResponseCode = 200
	toilet.ResponseBody = "Thank you for this dump. I hope you have a lovely day!"
	toilet.DumpBodyFirst = false

	// Store this toilet
	if _, err := storeToilet(context, client, toilet, toiletID); err != nil {
		logError(context, "Failed creating new toilet", err)
		return nil, err
	}

	return toilet, nil
}

// Returns true if a toilet is invalid
// Currently this is hackily defined as having a ResponseCode of -1
func isBlockedToilet(toilet *Toilet) bool {
	return toilet.ResponseCode == -1
}
