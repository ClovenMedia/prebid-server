package clovenmedia

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/prebid/openrtb/v19/openrtb2"
	"github.com/prebid/prebid-server/adapters"
	"github.com/prebid/prebid-server/config"
	"github.com/prebid/prebid-server/errortypes"
	"github.com/prebid/prebid-server/openrtb_ext"
)

type adapter struct {
	endpoint string
}

func Builder(bidderName openrtb_ext.BidderName, config config.Adapter, server config.Server) (adapters.Bidder, error) {
	bidder := &adapter{
		endpoint: config.Endpoint,
	}
	return bidder, nil
}

func (a *adapter) MakeRequests(request *openrtb2.BidRequest, requestInfo *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, []error{err}
	}

	requestData := &adapters.RequestData{
		Method: "POST",
		Uri:    a.endpoint,
		Body:   requestJSON,
	}

	return []*adapters.RequestData{requestData}, nil
}

func getMediaTypeForBid(bid openrtb2.Bid) (openrtb_ext.BidType, error) {
	if bid.Ext != nil {
		var bidExt openrtb_ext.ExtBid
		err := json.Unmarshal(bid.Ext, &bidExt)
		if err == nil && bidExt.Prebid != nil {
			return openrtb_ext.ParseBidType(string(bidExt.Prebid.Type))
		}
	}

	return "", &errortypes.BadServerResponse{
		Message: fmt.Sprintf("Failed to parse impression \"%s\" mediatype", bid.ImpID),
	}
}

func (a *adapter) MakeBids(request *openrtb2.BidRequest, requestData *adapters.RequestData, responseData *adapters.ResponseData) (*adapters.BidderResponse, []error) {
	if responseData.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	if responseData.StatusCode == http.StatusBadRequest {
		err := &errortypes.BadInput{
			Message: "Unexpected status code: 400. Bad request from publisher. Run with request.debug = 1 for more info.",
		}
		return nil, []error{err}
	}

	if responseData.StatusCode != http.StatusOK {
		err := &errortypes.BadServerResponse{
			Message: fmt.Sprintf("Unexpected status code: %d. Run with request.debug = 1 for more info.", responseData.StatusCode),
		}
		return nil, []error{err}
	}

	var response openrtb2.BidResponse
	if err := json.Unmarshal(responseData.Body, &response); err != nil {
		return nil, []error{err}
	}

	bidResponse := adapters.NewBidderResponseWithBidsCapacity(len(request.Imp))
	bidResponse.Currency = response.Cur
	var errors []error
	for _, seatBid := range response.SeatBid {
		for i, bid := range seatBid.Bid {
			bidType, err := getMediaTypeForBid(bid)
			if err != nil {
				errors = append(errors, err)
				continue
			}
			bidResponse.Bids = append(bidResponse.Bids, &adapters.TypedBid{
				Bid:     &seatBid.Bid[i],
				BidType: bidType,
			})
		}
	}
	return bidResponse, nil
}
