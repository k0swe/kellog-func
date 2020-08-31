package kellog

import (
	"github.com/Matir/adifparser"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/xylo04/kellog-qrz-sync/generated/adifpb"
	"io"
	"log"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func adifToJson(adifString string) (*adifpb.Adif, error) {
	reader := adifparser.NewADIFReader(strings.NewReader(adifString))
	adi := new(adifpb.Adif)
	created, _ := ptypes.TimestampProto(time.Now())
	adi.Header = &adifpb.Header{
		AdifVersion:      "3.1.0",
		CreatedTimestamp: created,
		ProgramId:        "kellog-func",
		ProgramVersion:   "0.0.1",
	}
	qsos := make([]*adifpb.Qso, reader.RecordCount())
	record, err := reader.ReadRecord()
	for err == nil {
		qsos = append(qsos, recordToQso(record))
		record, err = reader.ReadRecord()
	}
	if err != io.EOF {
		return nil, err
	}
	adi.Qsos = qsos
	return adi, nil
}

func recordToQso(record adifparser.ADIFRecord) *adifpb.Qso {
	qso := new(adifpb.Qso)
	parseTopLevel(record, qso)
	parseAppDefined(record, qso)
	parseContactedStation(record, qso)
	parseLoggingStation(record, qso)
	parseContest(record, qso)
	parseUploads(record, qso)
	return qso
}

func parseTopLevel(record adifparser.ADIFRecord, qso *adifpb.Qso) {
	qso.Band, _ = record.GetValue("band")
	qso.BandRx, _ = record.GetValue("band_rx")
	qso.Comment, _ = record.GetValue("comment")
	qso.DistanceKm = getUint32(record, "distance")
	qso.Freq = getFloat64(record, "freq")
	qso.FreqRx = getFloat64(record, "freq_rx")
	qso.Mode, _ = record.GetValue("mode")
	qso.Notes, _ = record.GetValue("notes")
	qso.PublicKey, _ = record.GetValue("public_key")
	qso.Complete, _ = record.GetValue("qso_complete")
	qso.TimeOn = getTimestamp(record, "qso_date", "time_on")
	qso.TimeOff = getTimestamp(record, "qso_date_off", "time_off")
	qso.Random = getBool(record, "random")
	qso.RstReceived, _ = record.GetValue("rst_rcvd")
	qso.RstSent, _ = record.GetValue("rst_sent")
	qso.Submode, _ = record.GetValue("submode")
	qso.Swl = getBool(record, "swl")
}

func parseAppDefined(record adifparser.ADIFRecord, qso *adifpb.Qso) {
	qso.AppDefined = map[string]string{}
	setAppDefined(record, "app_qrzlog_logid", qso)
	setAppDefined(record, "app_qrzlog_qsldate", qso)
	setAppDefined(record, "app_qrzlog_status", qso)
	setAppDefined(record, "app_eqsl_ag", qso)
}

func parseContactedStation(record adifparser.ADIFRecord, qso *adifpb.Qso) {
	qso.ContactedStation = new(adifpb.Station)
	qso.ContactedStation.Address, _ = record.GetValue("address")
	qso.ContactedStation.Age = getUint32(record, "age")
	qso.ContactedStation.StationCall, _ = record.GetValue("call")
	qso.ContactedStation.County, _ = record.GetValue("cnty")
	qso.ContactedStation.Continent, _ = record.GetValue("cont")
	qso.ContactedStation.OpCall, _ = record.GetValue("contacted_op")
	qso.ContactedStation.Country, _ = record.GetValue("country")
	qso.ContactedStation.CqZone = getUint32(record, "cqz")
	qso.ContactedStation.DarcDok, _ = record.GetValue("darc_dok")
	qso.ContactedStation.Dxcc = getUint32(record, "dxcc")
	qso.ContactedStation.Email, _ = record.GetValue("email")
	qso.ContactedStation.OwnerCall, _ = record.GetValue("eq_call")
	qso.ContactedStation.Fists = getUint32(record, "fists")
	qso.ContactedStation.FistsCc = getUint32(record, "fists_cc")
	qso.ContactedStation.GridSquare, _ = record.GetValue("gridsquare")
	qso.ContactedStation.Iota, _ = record.GetValue("iota")
	qso.ContactedStation.IotaIslandId = getUint32(record, "iota_island_id")
	qso.ContactedStation.ItuZone = getUint32(record, "ituz")
	qso.ContactedStation.Latitude = getLatLon(record, "lat")
	qso.ContactedStation.Longitude = getLatLon(record, "lon")
	qso.ContactedStation.OpName, _ = record.GetValue("name")
	qso.ContactedStation.Pfx, _ = record.GetValue("pfx")
	qso.ContactedStation.QslVia, _ = record.GetValue("qsl_via")
	qso.ContactedStation.City, _ = record.GetValue("qth")
	qso.ContactedStation.Region, _ = record.GetValue("region")
	qso.ContactedStation.Rig, _ = record.GetValue("rig")
	qso.ContactedStation.Power = getFloat64(record, "rx_pwr")
	qso.ContactedStation.Sig, _ = record.GetValue("sig")
	qso.ContactedStation.SigInfo, _ = record.GetValue("sig_info")
	qso.ContactedStation.SilentKey = getBool(record, "silent_key")
	qso.ContactedStation.Skcc, _ = record.GetValue("skcc")
	qso.ContactedStation.SotaRef, _ = record.GetValue("sota_ref")
	qso.ContactedStation.State, _ = record.GetValue("state")
	qso.ContactedStation.TenTen = getUint32(record, "ten_ten")
	qso.ContactedStation.Uksmg = getUint32(record, "uksmg")
	qso.ContactedStation.UsacaCounties, _ = record.GetValue("usaca_counties")
	qso.ContactedStation.VuccGrids, _ = record.GetValue("vucc_grids")
	qso.ContactedStation.Web, _ = record.GetValue("web")
}

func parseLoggingStation(record adifparser.ADIFRecord, qso *adifpb.Qso) {
	qso.LoggingStation = new(adifpb.Station)
	qso.LoggingStation.AntennaAzimuth = getInt32(record, "ant_az")
	qso.LoggingStation.AntennaElevation = getInt32(record, "ant_el")
	qso.LoggingStation.Antenna, _ = record.GetValue("my_antenna")
	qso.LoggingStation.City, _ = record.GetValue("my_city")
	qso.LoggingStation.County, _ = record.GetValue("my_cnty")
	qso.LoggingStation.CqZone = getUint32(record, "my_cq_zone")
	qso.LoggingStation.Dxcc = getUint32(record, "my_dxcc")
	qso.LoggingStation.Fists = getUint32(record, "my_fists")
	qso.LoggingStation.GridSquare, _ = record.GetValue("my_gridsquare")
	qso.LoggingStation.Iota, _ = record.GetValue("my_iota")
	qso.LoggingStation.IotaIslandId = getUint32(record, "my_iota_island_id")
	qso.LoggingStation.ItuZone = getUint32(record, "my_itu_zone")
	qso.LoggingStation.Latitude = getLatLon(record, "my_lat")
	qso.LoggingStation.Longitude = getLatLon(record, "my_lon")
	qso.LoggingStation.OpName, _ = record.GetValue("my_name")
	qso.LoggingStation.PostalCode, _ = record.GetValue("my_postal_code")
	qso.LoggingStation.Rig, _ = record.GetValue("my_rig")
	qso.LoggingStation.Sig, _ = record.GetValue("my_sig")
	qso.LoggingStation.SigInfo, _ = record.GetValue("my_sig_info")
	qso.LoggingStation.SotaRef, _ = record.GetValue("my_sota_ref")
	qso.LoggingStation.State, _ = record.GetValue("my_state")
	qso.LoggingStation.Street, _ = record.GetValue("my_street")
	qso.LoggingStation.UsacaCounties, _ = record.GetValue("my_usaca_counties")
	qso.LoggingStation.VuccGrids, _ = record.GetValue("my_vucc_grids")
	qso.LoggingStation.OpCall, _ = record.GetValue("operator")
	qso.LoggingStation.OwnerCall, _ = record.GetValue("owner_callsign")
	qso.LoggingStation.StationCall, _ = record.GetValue("station_callsign")
	qso.LoggingStation.Power = getFloat64(record, "tx_pwr")
}

func parseContest(record adifparser.ADIFRecord, qso *adifpb.Qso) {
	contestId, _ := record.GetValue("contest_id")
	if contestId != "" {
		qso.Contest = new(adifpb.ContestData)
		qso.Contest.ContestId = contestId
		qso.Contest.ArrlSection, _ = record.GetValue("arrl_sect")
		qso.Contest.StationClass, _ = record.GetValue("class")
		qso.Contest.Check, _ = record.GetValue("check")
		qso.Contest.Precedence, _ = record.GetValue("precedence")
		qso.Contest.SerialReceived, _ = record.GetValue("srx")
		if qso.Contest.SerialReceived == "" {
			qso.Contest.SerialReceived, _ = record.GetValue("srx_string")
		}
		qso.Contest.SerialSent, _ = record.GetValue("stx")
		if qso.Contest.SerialSent == "" {
			qso.Contest.SerialSent, _ = record.GetValue("stx_string")
		}
	}
}

func parseUploads(record adifparser.ADIFRecord, qso *adifpb.Qso) {
	qrzStatus, _ := record.GetValue("qrzcom_qso_upload_status")
	if qrzStatus != "" {
		qso.Qrzcom = new(adifpb.Upload)
		qso.Qrzcom.UploadStatus = translateUploadStatus(qrzStatus)
		qso.Qrzcom.UploadDate = getDate(record, "qrzcom_qso_upload_date")
	}

	hrdStatus, _ := record.GetValue("hrdlog_qso_upload_status")
	if hrdStatus != "" {
		qso.Hrdlog = new(adifpb.Upload)
		qso.Hrdlog.UploadStatus = translateUploadStatus(hrdStatus)
		qso.Hrdlog.UploadDate = getDate(record, "hrdlog_qso_upload_date")
	}

	clublogStatus, _ := record.GetValue("clublog_qso_upload_status")
	if clublogStatus != "" {
		qso.Clublog = new(adifpb.Upload)
		qso.Clublog.UploadStatus = translateUploadStatus(clublogStatus)
		qso.Clublog.UploadDate = getDate(record, "clublog_qso_upload_date")
	}
}

func translateUploadStatus(status string) adifpb.UploadStatus {
	switch status {
	case "Y":
		return adifpb.UploadStatus_UPLOAD_COMPLETE
	case "N":
		return adifpb.UploadStatus_DO_NOT_UPLOAD
	case "M":
		return adifpb.UploadStatus_MODIFIED_AFTER_UPLOAD
	default:
		return adifpb.UploadStatus_UNKNOWN
	}
}

func setAppDefined(record adifparser.ADIFRecord, field string, qso *adifpb.Qso) {
	value, _ := record.GetValue(field)
	if value != "" {
		qso.AppDefined[field] = value
	}
}

func getLatLon(record adifparser.ADIFRecord, field string) float64 {
	st, _ := record.GetValue(field)
	r := regexp.MustCompile(`([NESW])(\d+) ([\d.]+)`)
	groups := r.FindStringSubmatch(st)
	cardinal := groups[1]
	degrees, _ := strconv.ParseFloat(groups[2], 64)
	minutes, _ := strconv.ParseFloat(groups[3], 64)
	retval := degrees + (minutes / 60.0)
	if cardinal == "S" || cardinal == "W" {
		retval *= -1
	}
	// 4 decimal places is enough; https://xkcd.com/2170/
	retval = math.Round(retval*10000) / 10000
	return retval
}

func getBool(record adifparser.ADIFRecord, field string) bool {
	st, _ := record.GetValue(field)
	return st == "Y"
}

func getFloat64(record adifparser.ADIFRecord, field string) float64 {
	st, _ := record.GetValue(field)
	fl, _ := strconv.ParseFloat(st, 64)
	return fl
}

func getUint32(record adifparser.ADIFRecord, field string) uint32 {
	s, _ := record.GetValue(field)
	i64, _ := strconv.ParseUint(s, 10, 32)
	return uint32(i64)
}

func getInt32(record adifparser.ADIFRecord, field string) int32 {
	s, _ := record.GetValue(field)
	i64, _ := strconv.ParseInt(s, 10, 32)
	return int32(i64)
}

func getTimestamp(record adifparser.ADIFRecord, dateField string, timeField string) *timestamp.Timestamp {
	dateStr, _ := record.GetValue(dateField)
	timeStr, _ := record.GetValue(timeField)
	t, err := time.Parse("20060102 1504", dateStr+" "+timeStr)
	if err != nil {
		log.Print(err)
	}
	ts, err := ptypes.TimestampProto(t)
	if err != nil {
		log.Print(err)
	}
	return ts
}

func getDate(record adifparser.ADIFRecord, field string) *timestamp.Timestamp {
	dateStr, _ := record.GetValue(field)
	t, err := time.Parse("20060102", dateStr)
	if err != nil {
		log.Print(err)
	}
	ts, err := ptypes.TimestampProto(t)
	if err != nil {
		log.Print(err)
	}
	return ts
}