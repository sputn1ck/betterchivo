package swaprpc

import (
	context "context"
	"errors"
	"time"
)

type SwapServerService interface {
	GetRates() (*GetRatesResponse, error)
	StartReceiveSwap(*ReceiveRequestMessage) (*WaitForPaymentMessage, <-<-chan *CompleteMessage, error)
	CancelReceiveSwap(message *CancelMessage) (*CompleteMessage, error)

	StartPaySwap(*PaymentRequestmessage) (*PayAgreementMessage,error)

}

type PaymentReceived error

type SwapServer struct {
	swap SwapServerService
	UnimplementedSwapServiceServer
}

func (s *SwapServer) GetRates(ctx context.Context, request *GetRatesRequest) (*GetRatesResponse, error) {
	return s.swap.GetRates()
}

func (s *SwapServer) SendPayment(server SwapService_SendPaymentServer) error {
	recv, err := server.Recv()
	if err != nil {
		return err
	}
	payreq := recv.GetPaymentRequest()
	if payreq == nil {
		return errors.New("send flow must start with payment request")
	}

	agreement, err := s.swap.StartPaySwap(payreq)
	if err != nil {
		return err
	}
	err = server.Send(&SendPaymentResponse{PaymentId: recv.PaymentId, Message: &SendPaymentResponse_PayAgreement{agreement}})
	if err != nil {
		return err
	}
	recv, err := server.Recv()
	if err != nil {
		return err
	}

}

func (s *SwapServer) ReceivePayment(server SwapService_ReceivePaymentServer) error {
	cancelChan := make(chan *CancelMessage)
	recv, err := server.Recv()
	if err != nil {
		return err
	}
	req := recv.GetReceiveRequest()
	if req == nil {
		return errors.New("receive flow must start with receive request")
	}
	go func() {
		recv, err := server.Recv()
		if err != nil {
			return
		}
		cancel := recv.GetCancel()
		if cancel != nil {
			cancelChan <- cancel
		}
	}()

	waitForPaymentMessage, completeMessageChan, err := s.swap.StartReceiveSwap(req)
	if err != nil {
		return err
	}

	err = server.Send(&ReceivePaymentResponse{PaymentId: recv.PaymentId, Message: &ReceivePaymentResponse_WaitForPayment{waitForPaymentMessage}})
	if err != nil {
		return err
	}
	for {
		select {
			case <-server.Context().Done():
				return errors.New("context done")
			case cancelMsg := <-cancelChan:
				completeMsg, err := s.swap.CancelReceiveSwap(cancelMsg)
				if err != nil {
					return err
				}
				err = server.Send(&ReceivePaymentResponse{PaymentId: recv.PaymentId, Message: &ReceivePaymentResponse_Complete{completeMsg}})
				if err != nil {
					return err
				}
			case completeMessage := <-completeMessageChan:
				err = server.Send(&ReceivePaymentResponse{PaymentId: recv.PaymentId, Message: &ReceivePaymentResponse_Complete{completeMessage}})
				if err != nil {
					return err
				}
				return nil
		default:
			time.Sleep(100*time.Millisecond)
		}
	}
}

