package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	terrors "github.com/mondough/typhon/errors"
	"github.com/mondough/typhon/mock"
	"github.com/mondough/typhon/rabbit"
	"github.com/obeattie/mercury"
	"github.com/obeattie/mercury/client"
	"github.com/obeattie/mercury/marshaling"
	"github.com/obeattie/mercury/server"
	"github.com/obeattie/mercury/testproto"
	"github.com/obeattie/mercury/transport"
)

const testServiceName = "service.client-server-example"

func TestClientServerSuite_MockTransport(t *testing.T) {
	suite.Run(t, &clientServerSuite{
		TransF: func() transport.Transport {
			return mock.NewTransport()
		}})
}

func TestClientServerSuite_RabbitTransport(t *testing.T) {
	suite.Run(t, &clientServerSuite{
		TransF: func() transport.Transport {
			return rabbit.NewTransport()
		}})
}

type clientServerSuite struct {
	suite.Suite
	TransF func() transport.Transport
	trans  transport.Transport
	server server.Server
}

func (suite *clientServerSuite) SetupSuite() {
	trans := suite.TransF()
	select {
	case <-trans.Ready():
	case <-time.After(2 * time.Second):
		panic("transport not ready")
	}
	suite.trans = trans
}

func (suite *clientServerSuite) SetupTest() {
	suite.server = server.NewServer(testServiceName)
	suite.server.SetMiddleware(DefaultServerMiddleware())
	suite.server.Start(suite.trans)
}

func (suite *clientServerSuite) TearDownTest() {
	suite.server.Stop()
	suite.server = nil
}

func (suite *clientServerSuite) TearDownSuite() {
	suite.trans.Tomb().Killf("Test ending")
	suite.trans.Tomb().Wait()
	suite.trans = nil
}

func (suite *clientServerSuite) TestE2E() {
	suite.server.AddEndpoints(
		server.Endpoint{
			Name:     "test",
			Request:  new(testproto.DummyRequest),
			Response: new(testproto.DummyResponse),
			Handler: func(req mercury.Request) (mercury.Response, error) {
				return req.Response(&testproto.DummyResponse{
					Pong: "teste2e",
				}), nil
			}})

	cl := client.NewClient().
		SetMiddleware(DefaultClientMiddleware()).
		Add(
		client.Call{
			Uid:      "call",
			Service:  testServiceName,
			Endpoint: "test",
			Body:     &testproto.DummyRequest{},
			Response: &testproto.DummyResponse{},
		}).
		SetTransport(suite.trans).
		SetTimeout(time.Second).
		Execute()

	suite.Assert().False(cl.Errors().Any())
	rsp := cl.Response("call")
	suite.Assert().NotNil(rsp)
	response := rsp.Body().(*testproto.DummyResponse)
	suite.Assert().Equal("teste2e", response.Pong)
}

// TestErrors verifies that an error sent from a handler is correctly returned by a client
func (suite *clientServerSuite) TestErrors() {
	suite.server.AddEndpoints(server.Endpoint{
		Name:     "error",
		Request:  new(testproto.DummyRequest),
		Response: new(testproto.DummyResponse),
		Handler: func(req mercury.Request) (mercury.Response, error) {
			return nil, terrors.BadRequest("naughty naughty")
		}})

	cl := client.NewClient().
		SetMiddleware(DefaultClientMiddleware()).
		Add(
		client.Call{
			Uid:      "call",
			Service:  testServiceName,
			Endpoint: "error",
			Body:     &testproto.DummyRequest{},
			Response: &testproto.DummyResponse{},
		}).
		SetTransport(suite.trans).
		SetTimeout(time.Second).
		Execute()

	suite.Assert().True(cl.Errors().Any())
	err := cl.Errors().ForUid("call")
	suite.Assert().NotNil(err)
	suite.Assert().Equal(terrors.ErrBadRequest, err.Code)
}

// TestJSON verifies a JSON request and response can be received from a protobuf handler
func (suite *clientServerSuite) TestJSON() {
	suite.server.AddEndpoints(
		server.Endpoint{
			Name:     "test",
			Request:  new(testproto.DummyRequest),
			Response: new(testproto.DummyResponse),
			Handler: func(req mercury.Request) (mercury.Response, error) {
				request := req.Body().(*testproto.DummyRequest)
				return req.Response(&testproto.DummyResponse{
					Pong: request.Ping,
				}), nil
			}})

	req := mercury.NewRequest()
	req.SetService(testServiceName)
	req.SetEndpoint("test")
	req.SetPayload([]byte(`{ "ping": "blah blah blah" }`))
	req.SetHeader(marshaling.ContentTypeHeader, "application/json")
	req.SetHeader(marshaling.AcceptHeader, "application/json")

	cl := client.NewClient().
		SetMiddleware(DefaultClientMiddleware()).
		AddRequest("call", req).
		SetTransport(suite.trans).
		SetTimeout(time.Second).
		Execute()

	suite.Assert().False(cl.Errors().Any())
	rsp := cl.Response("call")
	suite.Assert().NotNil(rsp)
	var body map[string]string
	suite.Assert().NoError(json.Unmarshal(rsp.Payload(), &body))
	suite.Assert().NotNil(body)
	suite.Assert().Equal(1, len(body))
	suite.Assert().Equal("blah blah blah", body["pong"])
}
