package general

import (
	"bytes"
	"log"

	"git.andrewo.pw/andrew/ipod"
)

type DeviceGeneral interface {
	UIMode() UIMode
	SetUIMode(UIMode)
	Name() string
	SoftwareVersion() (major, minor, rev uint8)
	SerialNum() string

	LingoProtocolVersion(lingo uint8) (major, minor uint8)
	LingoOptions(ling uint8) uint64

	PrefSettingID(classID uint8) uint8
	SetPrefSettingID(classID, settingID uint8, restoreOnExit bool)

	StartIDPS()

	SetEventNotificationMask(mask uint64)
	EventNotificationMask() uint64
	SupportedEventNotificationMask() uint64

	CancelCommand(lingo uint8, cmd uint16, transaction uint16)

	MaxPayload() uint16
}

func ackSuccess(req ipod.Packet) ACK {
	return ACK{Status: ACKStatusSuccess, CmdID: req.ID.CmdID()}
}

func ackPending(req ipod.Packet, maxWait uint32) ACKPending {
	return ACKPending{Status: ACKStatusPending, CmdID: req.ID.CmdID(), MaxWait: maxWait}
}

func ackFIDTokens(tokens SetFIDTokenValues) RetFIDTokenValueACKs {
	resp := RetFIDTokenValueACKs{NumFIDTokenValueACKs: tokens.NumFIDTokenValues}
	buf := bytes.Buffer{}
	for _, token := range tokens.FIDTokenValues {
		switch token.FIDType {
		case 0x00:
			switch token.FIDSubtype {
			case 0x00:
				//identify
				buf.Write([]byte{0x03, 0x00, 0x00, 0x00})
			case 0x01:
				//acc caps
				buf.Write([]byte{0x03, 0x00, 0x01, 0x00})
			case 0x02:
				//accinfo
				buf.Write([]byte{0x04, 0x00, 0x02, 0x00})
			case 0x03:
				//ipod pref
				//check
				buf.Write([]byte{0x04, 0x00, 0x03, 0x00, 0x00})
			case 0x04:
				//ea proto
				//check
				buf.Write([]byte{0x04, 0x00, 0x04, 0x00, 0x00})
			case 0x05:
				// bundleseed
				buf.Write([]byte{0x03, 0x00, 0x05, 0x00})
			case 0x07:
				// screen info
				buf.Write([]byte{0x03, 0x00, 0x07, 0x00})
			case 0x08:
				// eaprotometadata
				buf.Write([]byte{0x03, 0x00, 0x08, 0x00})
			}
		case 0x01:
			//mic
			buf.Write([]byte{0x03, 0x01, 0x00, 0x00})
		}
	}
	resp.FIDTokenValueACKs = buf.Bytes()
	return resp
}

func HandleGeneral(req ipod.Packet, tr ipod.PacketWriter, dev DeviceGeneral) error {
	log.Printf("Req: %#v", req)
	switch msg := req.Payload.(type) {
	case RequestRemoteUIMode:
		ipod.Respond(req, tr, ReturnRemoteUIMode{
			Mode: ipod.BoolToByte(dev.UIMode() == UIModeExtended),
		})
	case EnterRemoteUIMode:
		if dev.UIMode() == UIModeExtended {
			ipod.Respond(req, tr, ackSuccess(req))
		} else {
			ipod.Respond(req, tr, ackPending(req, 300))
			dev.SetUIMode(UIModeExtended)
			ipod.Respond(req, tr, ackSuccess(req))
		}
	case ExitRemoteUIMode:
		if dev.UIMode() != UIModeExtended {
			ipod.Respond(req, tr, ackSuccess(req))
		} else {
			ipod.Respond(req, tr, ackPending(req, 300))
			dev.SetUIMode(UIModeStandart)
			ipod.Respond(req, tr, ackSuccess(req))
		}
	case RequestiPodName:
		ipod.Respond(req, tr, ReturniPodName{Name: ipod.StringToBytes(dev.Name())})
	case RequestiPodSoftwareVersion:
		var resp ReturniPodSoftwareVersion
		resp.Major, resp.Minor, resp.Rev = dev.SoftwareVersion()
		ipod.Respond(req, tr, resp)
	case RequestiPodSerialNum:
		ipod.Respond(req, tr, ReturniPodSerialNum{Serial: ipod.StringToBytes(dev.SerialNum())})
	case RequestLingoProtocolVersion:
		var resp ReturnLingoProtocolVersion
		resp.Major, resp.Minor = dev.LingoProtocolVersion(msg.Lingo)
		ipod.Respond(req, tr, resp)
	case RequestTransportMaxPayloadSize:
		ipod.Respond(req, tr, ReturnTransportMaxPayloadSize{MaxPayload: dev.MaxPayload()})
	case IdentifyDeviceLingoes:
		ipod.Respond(req, tr, ackSuccess(req))

	//GetDevAuthenticationInfo
	case RetDevAuthenticationInfoV1:
		ipod.Respond(req, tr, AckDevAuthenticationInfo{Status: DevAuthInfoStatusSupported})
		// get signature
	case RetDevAuthenticationInfoV2:
		if msg.CertCurrentSection < msg.CertMaxSection {
			ipod.Respond(req, tr, ackSuccess(req))
		} else {
			ipod.Respond(req, tr, AckDevAuthenticationInfo{Status: DevAuthInfoStatusSupported})
			// get signature
		}

	// GetDevAuthenticationSignatureV1
	case RetDevAuthenticationSignatureV1:
		ipod.Respond(req, tr, AckDevAuthenticationStatus{Status: DevAuthStatusPassed})
	// GetDevAuthenticationSignatureV2
	case RetDevAuthenticationSignatureV2:
		ipod.Respond(req, tr, AckDevAuthenticationStatus{Status: DevAuthStatusPassed})

	case GetiPodAuthenticationInfo:
		ipod.Respond(req, tr, RetiPodAuthenticationInfo{
			Major: 1, Minor: 1,
			CertCurrentSection: 0, CertMaxSection: 0, CertData: []byte{},
		})

	case AckiPodAuthenticationInfo:
		// pass

	case GetiPodAuthenticationSignature:
		ipod.Respond(req, tr, RetiPodAuthenticationSignature{Signature: msg.Challenge})

	case AckiPodAuthenticationStatus:
		// pass

	// revisit
	case GetiPodOptions:
		ipod.Respond(req, tr, RetiPodOptions{Options: 0x00})

	// GetAccessoryInfo
	// check back might be useful
	case RetAccessoryInfo:
		// pass

	case GetiPodPreferences:
		ipod.Respond(req, tr, RetiPodPreferences{
			PrefClassID:        msg.PrefClassID,
			PrefClassSettingID: dev.PrefSettingID(msg.PrefClassID),
		})

	case SetiPodPreferences:
		dev.SetPrefSettingID(msg.PrefClassID, msg.PrefClassSettingID, ipod.ByteToBool(msg.RestoreOnExit))

	case GetUIMode:
		ipod.Respond(req, tr, RetUIMode{UIMode: dev.UIMode()})
	case SetUIMode:
		ipod.Respond(req, tr, ackSuccess(req))

	case StartIDPS:
		dev.StartIDPS()
		ipod.Respond(req, tr, ackSuccess(req))
	case SetFIDTokenValues:
		log.Printf("set fid token!")
		ipod.Respond(req, tr, ackFIDTokens(msg))
	case EndIDPS:
		switch msg.AccEndIDPSStatus {
		case AccEndIDPSStatusContinue:
			ipod.Respond(req, tr, IDPSStatus{Status: IDPSStatusOK})
			*req.Transaction = 0x0001
			ipod.Respond(req, tr, GetDevAuthenticationInfo{})

			// get dev auth info
		case AccEndIDPSStatusReset:
			ipod.Respond(req, tr, IDPSStatus{Status: IDPSStatusTimeLimitNotExceeded})
		case AccEndIDPSStatusAbandon:
			ipod.Respond(req, tr, IDPSStatus{Status: IDPSStatusWillNotAccept})
		case AccEndIDPSStatusNewLink:
			//pass
		}

	// SetAccStatusNotification, RetAccStatusNotification
	case AccessoryStatusNotification:

	// iPodNotification later
	case SetEventNotification:
		dev.SetEventNotificationMask(msg.EventMask)
		ipod.Respond(req, tr, ackSuccess(req))

	case GetiPodOptionsForLingo:
		ipod.Respond(req, tr, RetiPodOptionsForLingo{
			LingoID: msg.LingoID,
			Options: dev.LingoOptions(msg.LingoID),
		})

	case GetEventNotification:
		ipod.Respond(req, tr, RetEventNotification{
			EventMask: dev.EventNotificationMask(),
		})

	case GetSupportedEventNotification:
		ipod.Respond(req, tr, RetSupportedEventNotification{
			EventMask: dev.SupportedEventNotificationMask(),
		})

	case CancelCommand:
		dev.CancelCommand(msg.LingoID, msg.CmdID, msg.TransactionID)
		ipod.Respond(req, tr, ackSuccess(req))

	case SetAvailableCurrent:
		// notify acc

	case RequestApplicationLaunch:
		ipod.Respond(req, tr, ackSuccess(req))

	case GetNowPlayingFocusApp:
		ipod.Respond(req, tr, RetNowPlayingFocusApp{AppID: ipod.StringToBytes("")})

	default:
		_ = msg
	}
	return nil
}