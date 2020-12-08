package kellog

import (
	"cloud.google.com/go/firestore"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"fmt"
	"github.com/golang/protobuf/ptypes"
	"github.com/imdario/mergo"
	"github.com/jinzhu/copier"
	adifpb "github.com/k0swe/adif-json-protobuf/go"
	"golang.org/x/oauth2"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type FirestoreQso struct {
	qsopb  *adifpb.Qso
	docref *firestore.DocumentRef
}

type FirebaseManager struct {
	ctx             *context.Context
	userToken       *auth.Token
	firestoreClient *firestore.Client
	userDoc         *firestore.DocumentRef
	contactsCol     *firestore.CollectionRef
}

// Do a bunch of initialization. Verify JWT and get user token back, and init a Firestore connection
//as that user.
func MakeFirebaseManager(ctx *context.Context, r *http.Request) (*FirebaseManager, error) {
	// Use the application default credentials

	if projectID == "" {
		panic("GCP_PROJECT is not set")
	}
	conf := &firebase.Config{ProjectID: projectID}
	app, err := firebase.NewApp(*ctx, conf)
	if err != nil {
		// 500
		return nil, fmt.Errorf("error initializing Firebase app: %v", err)
	}

	authClient, err := app.Auth(*ctx)
	if err != nil {
		// 500
		return nil, fmt.Errorf("error getting authClient: %v", err)
	}
	idToken, err := extractIdToken(r)
	if err != nil {
		// 403
		return nil, fmt.Errorf("couldn't find authorization: %v", err)
	}
	userToken, err := authClient.VerifyIDToken(*ctx, idToken)
	if err != nil {
		// 403
		return nil, fmt.Errorf("couldn't verify authorization: %v", err)
	}
	firestoreClient, err := makeFirestoreClient(*ctx, idToken)
	if err != nil {
		// 500
		return nil, fmt.Errorf("error creating firestore client: %v", err)
	}
	userDoc := firestoreClient.Collection("users").Doc(userToken.UID)
	return &FirebaseManager{
		ctx,
		userToken,
		firestoreClient,
		userDoc,
		userDoc.Collection("contacts"),
	}, nil
}

func extractIdToken(r *http.Request) (string, error) {
	idToken := strings.TrimSpace(r.Header.Get("Authorization"))
	if idToken == "" {
		return "", errors.New("requests must be authenticated with a Firebase JWT")
	}
	idToken = strings.TrimPrefix(idToken, "Bearer ")
	return idToken, nil
}

func makeFirestoreClient(ctx context.Context, idToken string) (*firestore.Client, error) {
	conf := &firebase.Config{ProjectID: projectID}
	userApp, err := firebase.NewApp(ctx, conf, option.WithTokenSource(
		oauth2.StaticTokenSource(
			&oauth2.Token{
				AccessToken: idToken,
			})))
	if err != nil {
		return nil, err
	}
	firestoreClient, err := userApp.Firestore(ctx)
	if err != nil {
		return nil, err
	}
	return firestoreClient, nil
}

func (f *FirebaseManager) GetUID() string {
	return f.userToken.UID
}

func (f *FirebaseManager) GetUserSetting(key string) (string, error) {
	userSettings, err := f.getUserSettings()
	if err != nil {
		return "", err
	}
	return fmt.Sprint(userSettings[key]), nil
}

func (f *FirebaseManager) getUserSettings() (map[string]interface{}, error) {
	// This could be memoized, but I think the Firestore client does that anyway
	userSettings, err := f.userDoc.Get(*f.ctx)
	if err != nil {
		return nil, err
	}
	if !userSettings.Exists() {
		return make(map[string]interface{}), nil
	}
	return userSettings.Data(), nil
}

func (f *FirebaseManager) GetContacts() ([]FirestoreQso, error) {
	docItr := f.contactsCol.Documents(*f.ctx)
	var retval = make([]FirestoreQso, 0, 100)
	for i := 0; ; i++ {
		qsoDoc, err := docItr.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		// I want to just qsoDoc.DataTo(&qso), but timestamps don't unmarshal
		buf := qsoDoc.Data()
		marshal, _ := json.Marshal(buf)
		var qso adifpb.Qso
		err = protojson.Unmarshal(marshal, &qso)
		if err != nil {
			log.Printf("Skipping qso %d: unmarshaling error: %v", i, err)
			continue
		}
		retval = append(retval, FirestoreQso{&qso, qsoDoc.Ref})
	}
	return retval, nil
}

// Merge the remote ADIF contacts into the Firestore ones. Returns the counts of
// QSOs created, modified, and with no difference.
func (f *FirebaseManager) MergeQsos(
	firebaseQsos []FirestoreQso,
	remoteAdi *adifpb.Adif) (int, int, int) {
	var created = 0
	var modified = 0
	var noDiff = 0
	m := map[string]FirestoreQso{}
	for _, fsQso := range firebaseQsos {
		hash := hashQso(fsQso.qsopb)
		m[hash] = fsQso
	}

	for _, remoteQso := range remoteAdi.Qsos {
		hash := hashQso(remoteQso)
		if _, ok := m[hash]; ok {
			diff := mergeQso(m[hash].qsopb, remoteQso)
			if diff {
				log.Printf("Updating QSO with %v on %v",
					remoteQso.ContactedStation.StationCall,
					remoteQso.TimeOn.String())
				err := f.Update(m[hash])
				if err != nil {
					continue
				}
				modified++
			} else {
				log.Printf("No difference for QSO with %v on %v",
					remoteQso.ContactedStation.StationCall,
					remoteQso.TimeOn.String())
				noDiff++
			}
		} else {
			log.Printf("Creating QSO with %v on %v",
				remoteQso.ContactedStation.StationCall,
				remoteQso.TimeOn.String())
			err := f.Create(remoteQso)
			if err != nil {
				continue
			}
			created++
		}
	}
	return created, modified, noDiff
}

func hashQso(qsopb *adifpb.Qso) string {
	timeOn, _ := ptypes.Timestamp(qsopb.TimeOn)
	// Some providers (QRZ.com) only have minute precision
	timeOn = timeOn.Truncate(time.Minute)
	payload := []byte(qsopb.LoggingStation.StationCall +
		qsopb.ContactedStation.StationCall +
		strconv.FormatInt(timeOn.Unix(), 10))
	return fmt.Sprintf("%x", sha256.Sum256(payload))
}

// Given two QSO objects, replace missing values in `base` with those from `backfill`. Values
// already present in `base` should be preserved.
func mergeQso(base *adifpb.Qso, backfill *adifpb.Qso) bool {
	original := &adifpb.Qso{}
	_ = copier.Copy(original, base)
	cleanQsl(base)
	cleanQsl(backfill)
	_ = mergo.Merge(base, backfill)
	return !proto.Equal(original, base)
}

func (f *FirebaseManager) Create(qso *adifpb.Qso) error {
	buf, err := qsoToJson(qso)
	if err != nil {
		log.Printf("Problem unmarshaling for create: %v", err)
		return err
	}
	_, err = f.contactsCol.NewDoc().Create(*f.ctx, buf)
	if err != nil {
		log.Printf("Problem creating: %v", err)
		return err
	}
	return nil
}

func (f *FirebaseManager) Update(qso FirestoreQso) error {
	buf, err := qsoToJson(qso.qsopb)
	if err != nil {
		log.Printf("Problem unmarshaling for update: %v", err)
		return err
	}
	_, err = qso.docref.Set(*f.ctx, buf)
	if err != nil {
		log.Printf("Problem updating: %v", err)
		return err
	}
	return nil
}

func qsoToJson(qso *adifpb.Qso) (map[string]interface{}, error) {
	jso, _ := protojson.Marshal(qso)
	var buf map[string]interface{}
	err := json.Unmarshal(jso, &buf)
	return buf, err
}

func cleanQsl(qso *adifpb.Qso) {
	// If QSO has LotW QSL with status=N or date=0001-01-01T00:00:00Z,
	// remove those to make way for the merge
	if qso.Lotw != nil {
		l := qso.Lotw
		if l.SentStatus == "N" {
			l.SentStatus = ""
		}
		if l.SentDate != nil &&
			(l.SentDate.Seconds == -62135596800 || l.SentDate.Seconds == 0) {
			l.SentDate = nil
		}
		if l.ReceivedStatus == "N" {
			l.ReceivedStatus = ""
		}
		if l.ReceivedDate != nil &&
			(l.ReceivedDate.Seconds == -62135596800 || l.ReceivedDate.Seconds == 0) {
			l.ReceivedDate = nil
		}
	}
}