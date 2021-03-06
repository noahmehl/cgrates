/*
Real-time Charging System for Telecom & ISP environments
Copyright (C) ITsysCOM GmbH

This program is free software: you can Storagetribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITH*out ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>
*/

package agents

import (
	"flag"
	"net/rpc"
	"net/rpc/jsonrpc"
	"path"
	"reflect"
	"testing"
	"time"

	"github.com/cgrates/cgrates/config"
	"github.com/cgrates/cgrates/engine"
	"github.com/cgrates/cgrates/sessionmanager"
	"github.com/cgrates/cgrates/utils"
	"github.com/fiorix/go-diameter/diam"
	"github.com/fiorix/go-diameter/diam/avp"
	"github.com/fiorix/go-diameter/diam/datatype"
)

var testIntegration = flag.Bool("integration", false, "Perform the tests in integration mode, not by default.") // This flag will be passed here via "go test -local" args
var waitRater = flag.Int("wait_rater", 100, "Number of miliseconds to wait for rater to start and cache")
var dataDir = flag.String("data_dir", "/usr/share/cgrates", "CGR data dir path here")

var daCfgPath string
var daCfg *config.CGRConfig
var apierRpc *rpc.Client
var dmtClient *DiameterClient
var err error

func TestDmtAgentInitCfg(t *testing.T) {
	if !*testIntegration {
		return
	}
	daCfgPath = path.Join(*dataDir, "conf", "samples", "dmtagent")
	// Init config first
	var err error
	daCfg, err = config.NewCGRConfigFromFolder(daCfgPath)
	if err != nil {
		t.Error(err)
	}
	daCfg.DataFolderPath = *dataDir // Share DataFolderPath through config towards StoreDb for Flush()
	config.SetCgrConfig(daCfg)
}

// Remove data in both rating and accounting db
func TestDmtAgentResetDataDb(t *testing.T) {
	if !*testIntegration {
		return
	}
	if err := engine.InitDataDb(daCfg); err != nil {
		t.Fatal(err)
	}
}

// Wipe out the cdr database
func TestDmtAgentResetStorDb(t *testing.T) {
	if !*testIntegration {
		return
	}
	if err := engine.InitStorDb(daCfg); err != nil {
		t.Fatal(err)
	}
}

// Start CGR Engine
func TestDmtAgentStartEngine(t *testing.T) {
	if !*testIntegration {
		return
	}
	if _, err := engine.StopStartEngine(daCfgPath, *waitRater); err != nil {
		t.Fatal(err)
	}
}

func TestDmtAgentCCRAsSMGenericEvent(t *testing.T) {
	if !*testIntegration {
		return
	}
	cfgDefaults, _ := config.NewDefaultCGRConfig()
	loadDictionaries(cfgDefaults.DiameterAgentCfg().DictionariesDir, "UNIT_TEST")
	ccr := &CCR{
		SessionId:         "routinga;1442095190;1476802709",
		OriginHost:        cfgDefaults.DiameterAgentCfg().OriginHost,
		OriginRealm:       cfgDefaults.DiameterAgentCfg().OriginRealm,
		DestinationHost:   cfgDefaults.DiameterAgentCfg().OriginHost,
		DestinationRealm:  cfgDefaults.DiameterAgentCfg().OriginRealm,
		AuthApplicationId: 4,
		ServiceContextId:  "voice@huawei.com",
		CCRequestType:     1,
		CCRequestNumber:   0,
		EventTimestamp:    time.Date(2015, 11, 23, 12, 22, 24, 0, time.UTC),
		ServiceIdentifier: 0,
		SubscriptionId: []struct {
			SubscriptionIdType int    `avp:"Subscription-Id-Type"`
			SubscriptionIdData string `avp:"Subscription-Id-Data"`
		}{
			struct {
				SubscriptionIdType int    `avp:"Subscription-Id-Type"`
				SubscriptionIdData string `avp:"Subscription-Id-Data"`
			}{SubscriptionIdType: 0, SubscriptionIdData: "4986517174963"},
			struct {
				SubscriptionIdType int    `avp:"Subscription-Id-Type"`
				SubscriptionIdData string `avp:"Subscription-Id-Data"`
			}{SubscriptionIdType: 0, SubscriptionIdData: "4986517174963"}},
		debitInterval: time.Duration(300) * time.Second,
	}
	ccr.RequestedServiceUnit.CCTime = 300
	ccr.ServiceInformation.INInformation.CallingPartyAddress = "4986517174963"
	ccr.ServiceInformation.INInformation.CalledPartyAddress = "4986517174964"
	ccr.ServiceInformation.INInformation.RealCalledNumber = "4986517174964"
	ccr.ServiceInformation.INInformation.ChargeFlowType = 0
	ccr.ServiceInformation.INInformation.CallingVlrNumber = "49123956767"
	ccr.ServiceInformation.INInformation.CallingCellIDOrSAI = "12340185301425"
	ccr.ServiceInformation.INInformation.BearerCapability = "capable"
	ccr.ServiceInformation.INInformation.CallReferenceNumber = "askjadkfjsdf"
	ccr.ServiceInformation.INInformation.MSCAddress = "123324234"
	ccr.ServiceInformation.INInformation.TimeZone = 0
	ccr.ServiceInformation.INInformation.CalledPartyNP = "4986517174964"
	ccr.ServiceInformation.INInformation.SSPTime = "20091020120101"
	var err error
	if ccr.diamMessage, err = ccr.AsDiameterMessage(); err != nil {
		t.Error(err)
	}
	eSMGE := sessionmanager.SMGenericEvent{"EventName": "DIAMETER_CCR", "AccId": "routinga;1442095190;1476802709",
		"Account": "*users", "AnswerTime": "2015-11-23 12:22:24 +0000 UTC", "Category": "call",
		"Destination": "4986517174964", "Direction": "*out", "ReqType": "*users", "SetupTime": "2015-11-23 12:22:24 +0000 UTC",
		"Subject": "*users", "SubscriberId": "4986517174963", "TOR": "*voice", "Tenant": "*users", "Usage": "300"}
	if smge, err := ccr.AsSMGenericEvent(cfgDefaults.DiameterAgentCfg().RequestProcessors[0].ContentFields); err != nil {
		t.Error(err)
	} else if !reflect.DeepEqual(eSMGE, smge) {
		t.Errorf("Expecting: %+v, received: %+v", eSMGE, smge)
	}
}

// Connect rpc client to rater
func TestDmtAgentApierRpcConn(t *testing.T) {
	if !*testIntegration {
		return
	}
	var err error
	apierRpc, err = jsonrpc.Dial("tcp", daCfg.RPCJSONListen) // We connect over JSON so we can also troubleshoot if needed
	if err != nil {
		t.Fatal(err)
	}
}

// Load the tariff plan, creating accounts and their balances
func TestDmtAgentTPFromFolder(t *testing.T) {
	if !*testIntegration {
		return
	}
	attrs := &utils.AttrLoadTpFromFolder{FolderPath: path.Join(*dataDir, "tariffplans", "tutorial")}
	var loadInst engine.LoadInstance
	if err := apierRpc.Call("ApierV2.LoadTariffPlanFromFolder", attrs, &loadInst); err != nil {
		t.Error(err)
	}
	time.Sleep(time.Duration(*waitRater) * time.Millisecond) // Give time for scheduler to execute topups
}

// cgr-console 'cost Category="call" Tenant="cgrates.org" Subject="1001" Destination="1004" TimeStart="2015-11-07T08:42:26Z" TimeEnd="2015-11-07T08:47:26Z"'
func TestDmtAgentSendCCRInit(t *testing.T) {
	if !*testIntegration {
		return
	}
	dmtClient, err = NewDiameterClient(daCfg.DiameterAgentCfg().Listen, "UNIT_TEST", daCfg.DiameterAgentCfg().OriginRealm,
		daCfg.DiameterAgentCfg().VendorId, daCfg.DiameterAgentCfg().ProductName, utils.DIAMETER_FIRMWARE_REVISION, daCfg.DiameterAgentCfg().DictionariesDir)
	if err != nil {
		t.Fatal(err)
	}
	cdr := &engine.StoredCdr{CgrId: utils.Sha1("dsafdsaf", time.Date(2015, 11, 7, 8, 42, 20, 0, time.UTC).String()), OrderId: 123, TOR: utils.VOICE,
		AccId: "dsafdsaf", CdrHost: "192.168.1.1", CdrSource: utils.UNIT_TEST, ReqType: utils.META_RATED, Direction: "*out",
		Tenant: "cgrates.org", Category: "call", Account: "1001", Subject: "1001", Destination: "1004", Supplier: "SUPPL1",
		SetupTime: time.Date(2015, 11, 7, 8, 42, 20, 0, time.UTC), AnswerTime: time.Date(2015, 11, 7, 8, 42, 26, 0, time.UTC), MediationRunId: utils.DEFAULT_RUNID,
		Usage: time.Duration(0) * time.Second, Pdd: time.Duration(7) * time.Second, ExtraFields: map[string]string{"Service-Context-Id": "voice@huawei.com"},
	}
	ccr := storedCdrToCCR(cdr, "UNIT_TEST", daCfg.DiameterAgentCfg().OriginRealm, daCfg.DiameterAgentCfg().VendorId,
		daCfg.DiameterAgentCfg().ProductName, utils.DIAMETER_FIRMWARE_REVISION, daCfg.DiameterAgentCfg().DebitInterval, false)
	m, err := ccr.AsDiameterMessage()
	if err != nil {
		t.Error(err)
	}
	if err := dmtClient.SendMessage(m); err != nil {
		t.Error(err)
	}
	time.Sleep(time.Duration(100) * time.Millisecond)
	var acnt *engine.Account
	attrs := &utils.AttrGetAccount{Tenant: "cgrates.org", Account: "1001"}
	eAcntVal := 9.484
	if err := apierRpc.Call("ApierV2.GetAccount", attrs, &acnt); err != nil {
		t.Error(err)
	} else if acnt.BalanceMap[utils.MONETARY].GetTotalValue() != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, acnt.BalanceMap[utils.MONETARY].GetTotalValue())
	}
}

// cgr-console 'cost Category="call" Tenant="cgrates.org" Subject="1001" Destination="1004" TimeStart="2015-11-07T08:42:26Z" TimeEnd="2015-11-07T08:52:26Z"'
func TestDmtAgentSendCCRUpdate(t *testing.T) {
	if !*testIntegration {
		return
	}
	cdr := &engine.StoredCdr{CgrId: utils.Sha1("dsafdsaf", time.Date(2015, 11, 7, 8, 42, 20, 0, time.UTC).String()), OrderId: 123, TOR: utils.VOICE,
		AccId: "dsafdsaf", CdrHost: "192.168.1.1", CdrSource: utils.UNIT_TEST, ReqType: utils.META_RATED, Direction: "*out",
		Tenant: "cgrates.org", Category: "call", Account: "1001", Subject: "1001", Destination: "1004", Supplier: "SUPPL1",
		SetupTime: time.Date(2015, 11, 7, 8, 42, 20, 0, time.UTC), AnswerTime: time.Date(2015, 11, 7, 8, 42, 26, 0, time.UTC), MediationRunId: utils.DEFAULT_RUNID,
		Usage: time.Duration(300) * time.Second, Pdd: time.Duration(7) * time.Second, ExtraFields: map[string]string{"Service-Context-Id": "voice@huawei.com"},
	}
	ccr := storedCdrToCCR(cdr, "UNIT_TEST", daCfg.DiameterAgentCfg().OriginRealm, daCfg.DiameterAgentCfg().VendorId,
		daCfg.DiameterAgentCfg().ProductName, utils.DIAMETER_FIRMWARE_REVISION, daCfg.DiameterAgentCfg().DebitInterval, false)
	m, err := ccr.AsDiameterMessage()
	if err != nil {
		t.Error(err)
	}
	if err := dmtClient.SendMessage(m); err != nil {
		t.Error(err)
	}
	time.Sleep(time.Duration(100) * time.Millisecond)
	var acnt *engine.Account
	attrs := &utils.AttrGetAccount{Tenant: "cgrates.org", Account: "1001"}
	eAcntVal := 9.214
	if err := apierRpc.Call("ApierV2.GetAccount", attrs, &acnt); err != nil {
		t.Error(err)
	} else if acnt.BalanceMap[utils.MONETARY].GetTotalValue() != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, acnt.BalanceMap[utils.MONETARY].GetTotalValue())
	}
}

// cgr-console 'cost Category="call" Tenant="cgrates.org" Subject="1001" Destination="1004" TimeStart="2015-11-07T08:42:26Z" TimeEnd="2015-11-07T08:57:26Z"'
func TestDmtAgentSendCCRUpdate2(t *testing.T) {
	if !*testIntegration {
		return
	}
	cdr := &engine.StoredCdr{CgrId: utils.Sha1("dsafdsaf", time.Date(2015, 11, 7, 8, 42, 20, 0, time.UTC).String()), OrderId: 123, TOR: utils.VOICE,
		AccId: "dsafdsaf", CdrHost: "192.168.1.1", CdrSource: utils.UNIT_TEST, ReqType: utils.META_RATED, Direction: "*out",
		Tenant: "cgrates.org", Category: "call", Account: "1001", Subject: "1001", Destination: "1004", Supplier: "SUPPL1",
		SetupTime: time.Date(2015, 11, 7, 8, 42, 20, 0, time.UTC), AnswerTime: time.Date(2015, 11, 7, 8, 42, 26, 0, time.UTC), MediationRunId: utils.DEFAULT_RUNID,
		Usage: time.Duration(600) * time.Second, Pdd: time.Duration(7) * time.Second, ExtraFields: map[string]string{"Service-Context-Id": "voice@huawei.com"},
	}
	ccr := storedCdrToCCR(cdr, "UNIT_TEST", daCfg.DiameterAgentCfg().OriginRealm, daCfg.DiameterAgentCfg().VendorId,
		daCfg.DiameterAgentCfg().ProductName, utils.DIAMETER_FIRMWARE_REVISION, daCfg.DiameterAgentCfg().DebitInterval, false)
	m, err := ccr.AsDiameterMessage()
	if err != nil {
		t.Error(err)
	}
	if err := dmtClient.SendMessage(m); err != nil {
		t.Error(err)
	}
	time.Sleep(time.Duration(100) * time.Millisecond)
	var acnt *engine.Account
	attrs := &utils.AttrGetAccount{Tenant: "cgrates.org", Account: "1001"}
	eAcntVal := 8.944
	if err := apierRpc.Call("ApierV2.GetAccount", attrs, &acnt); err != nil {
		t.Error(err)
	} else if acnt.BalanceMap[utils.MONETARY].GetTotalValue() != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, acnt.BalanceMap[utils.MONETARY].GetTotalValue())
	}
}

func TestDmtAgentSendCCRTerminate(t *testing.T) {
	if !*testIntegration {
		return
	}
	cdr := &engine.StoredCdr{CgrId: utils.Sha1("dsafdsaf", time.Date(2015, 11, 7, 8, 42, 20, 0, time.UTC).String()), OrderId: 123, TOR: utils.VOICE,
		AccId: "dsafdsaf", CdrHost: "192.168.1.1", CdrSource: utils.UNIT_TEST, ReqType: utils.META_RATED, Direction: "*out",
		Tenant: "cgrates.org", Category: "call", Account: "1001", Subject: "1001", Destination: "1004", Supplier: "SUPPL1",
		SetupTime: time.Date(2015, 11, 7, 8, 42, 20, 0, time.UTC), AnswerTime: time.Date(2015, 11, 7, 8, 42, 26, 0, time.UTC), MediationRunId: utils.DEFAULT_RUNID,
		Usage: time.Duration(610) * time.Second, Pdd: time.Duration(7) * time.Second, ExtraFields: map[string]string{"Service-Context-Id": "voice@huawei.com"},
	}
	ccr := storedCdrToCCR(cdr, "UNIT_TEST", daCfg.DiameterAgentCfg().OriginRealm, daCfg.DiameterAgentCfg().VendorId,
		daCfg.DiameterAgentCfg().ProductName, utils.DIAMETER_FIRMWARE_REVISION, daCfg.DiameterAgentCfg().DebitInterval, true)
	m, err := ccr.AsDiameterMessage()
	if err != nil {
		t.Error(err)
	}
	if err := dmtClient.SendMessage(m); err != nil {
		t.Error(err)
	}
	time.Sleep(time.Duration(100) * time.Millisecond)
	var acnt *engine.Account
	attrs := &utils.AttrGetAccount{Tenant: "cgrates.org", Account: "1001"}
	eAcntVal := 9.205
	if err := apierRpc.Call("ApierV2.GetAccount", attrs, &acnt); err != nil {
		t.Error(err)
	} else if acnt.BalanceMap[utils.MONETARY].GetTotalValue() != eAcntVal { // Should also consider derived charges which double the cost of 6m10s - 2x0.7584
		t.Errorf("Expected: %f, received: %f", eAcntVal, acnt.BalanceMap[utils.MONETARY].GetTotalValue())
	}
}

func TestDmtAgentCdrs(t *testing.T) {
	if !*testIntegration {
		return
	}
	var cdrs []*engine.ExternalCdr
	req := utils.RpcCdrsFilter{RunIds: []string{utils.META_DEFAULT}}
	if err := apierRpc.Call("ApierV2.GetCdrs", req, &cdrs); err != nil {
		t.Error("Unexpected error: ", err.Error())
	} else if len(cdrs) != 1 {
		t.Error("Unexpected number of CDRs returned: ", len(cdrs))
	} else {
		if cdrs[0].Usage != "610" {
			t.Errorf("Unexpected CDR Usage received, cdr: %+v ", cdrs[0])
		}
		if cdrs[0].Cost != 0.795 {
			t.Errorf("Unexpected CDR Cost received, cdr: %+v ", cdrs[0])
		}
	}
}

func TestDmtAgentHuaweiSim1(t *testing.T) {
	if !*testIntegration {
		return
	}
	m := diam.NewRequest(diam.CreditControl, 4, nil)
	m.NewAVP("Session-Id", avp.Mbit, 0, datatype.UTF8String("simuhuawei;1449573472;00002"))
	m.NewAVP("Origin-Host", avp.Mbit, 0, datatype.DiameterIdentity("simuhuawei"))
	m.NewAVP("Origin-Realm", avp.Mbit, 0, datatype.DiameterIdentity("routing1.huawei.com"))
	m.NewAVP("Destination-Host", avp.Mbit, 0, datatype.DiameterIdentity("CGR-DA"))
	m.NewAVP("Destination-Realm", avp.Mbit, 0, datatype.DiameterIdentity("cgrates.org"))
	m.NewAVP("Auth-Application-Id", avp.Mbit, 0, datatype.Unsigned32(4))
	m.NewAVP("Service-Context-Id", avp.Mbit, 0, datatype.UTF8String("voice@huawei.com"))
	m.NewAVP("CC-Request-Type", avp.Mbit, 0, datatype.Enumerated(1))
	m.NewAVP("CC-Request-Number", avp.Mbit, 0, datatype.Enumerated(0))
	m.NewAVP("Event-Timestamp", avp.Mbit, 0, datatype.Time(time.Now()))
	m.NewAVP("Subscription-Id", avp.Mbit, 0, &diam.GroupedAVP{
		AVP: []*diam.AVP{
			diam.NewAVP(450, avp.Mbit, 0, datatype.Enumerated(0)),             // Subscription-Id-Type
			diam.NewAVP(444, avp.Mbit, 0, datatype.UTF8String("33708000003")), // Subscription-Id-Data
		}})
	m.NewAVP("Subscription-Id", avp.Mbit, 0, &diam.GroupedAVP{
		AVP: []*diam.AVP{
			diam.NewAVP(450, avp.Mbit, 0, datatype.Enumerated(1)),              // Subscription-Id-Type
			diam.NewAVP(444, avp.Mbit, 0, datatype.UTF8String("208708000003")), // Subscription-Id-Data
		}})
	m.NewAVP("Service-Identifier", avp.Mbit, 0, datatype.Unsigned32(0))
	m.NewAVP("Requested-Service-Unit", avp.Mbit, 0, &diam.GroupedAVP{
		AVP: []*diam.AVP{
			diam.NewAVP(420, avp.Mbit, 0, datatype.Unsigned32(360))}}) // CC-Time
	m.NewAVP(873, avp.Mbit, 10415, &diam.GroupedAVP{
		AVP: []*diam.AVP{
			diam.NewAVP(20300, avp.Mbit, 2011, &diam.GroupedAVP{ // IN-Information
				AVP: []*diam.AVP{
					diam.NewAVP(831, avp.Mbit, 10415, datatype.UTF8String("33708000003")),           // Calling-Party-Address
					diam.NewAVP(832, avp.Mbit, 10415, datatype.UTF8String("780029555")),             // Called-Party-Address
					diam.NewAVP(20327, avp.Mbit, 2011, datatype.UTF8String("33780029555")),          // Real-Called-Number
					diam.NewAVP(20339, avp.Mbit, 2011, datatype.Unsigned32(0)),                      // Charge-Flow-Type
					diam.NewAVP(20302, avp.Mbit, 2011, datatype.UTF8String("33609")),                // Calling-Vlr-Number
					diam.NewAVP(20303, avp.Mbit, 2011, datatype.UTF8String("208102000018370")),      // Calling-CellID-Or-SAI
					diam.NewAVP(20313, avp.Mbit, 2011, datatype.UTF8String("80:90:a3")),             // Bearer-Capability
					diam.NewAVP(20321, avp.Mbit, 2011, datatype.UTF8String("40:04:41:31:06:46:18")), // Call-Reference-Number
					diam.NewAVP(20322, avp.Mbit, 2011, datatype.UTF8String("3333609")),              // MSC-Address
					diam.NewAVP(20324, avp.Mbit, 2011, datatype.Unsigned32(8)),                      // Time-Zone
					diam.NewAVP(20385, avp.Mbit, 2011, datatype.UTF8String("6002")),                 // Called-Party-NP
					diam.NewAVP(20386, avp.Mbit, 2011, datatype.UTF8String("20151208121752")),       // SSP-Time
				},
			}),
		}})
	if err := dmtClient.SendMessage(m); err != nil {
		t.Error(err)
	}
}

func TestDmtAgentStopEngine(t *testing.T) {
	if !*testIntegration {
		return
	}
	if err := engine.KillEngine(*waitRater); err != nil {
		t.Error(err)
	}
}
