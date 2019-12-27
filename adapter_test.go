package pgadapter

import (
	"os"
	"testing"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/util"
	"github.com/go-pg/pg/v9"
	"github.com/stretchr/testify/suite"
)

// AdapterTestSuite tests all functionalities of Adapter
type AdapterTestSuite struct {
	suite.Suite
	e *casbin.Enforcer
	a *Adapter
}

func (s *AdapterTestSuite) testGetPolicy(res [][]string) {
	s.T().Helper()
	myRes := s.e.GetPolicy()
	s.Assert().True(util.Array2DEquals(res, myRes), "Policy Got: %v, supposed to be %v", myRes, res)
}

func (s *AdapterTestSuite) dropCasbinDB() {
	opts, err := pg.ParseURL(os.Getenv("PG_CONN"))
	s.Require().NoError(err)
	db := pg.Connect(opts)
	defer db.Close()
	db.Exec("DROP DATABASE casbin")
}

func (s *AdapterTestSuite) SetupTest() {
	s.dropCasbinDB()

	var err error
	s.a, err = NewAdapter(os.Getenv("PG_CONN"))
	s.Require().NoError(err)

	// This is a trick to save the current policy to the DB.
	// We can't call e.SavePolicy() because the adapter in the enforcer is still the file adapter.
	// The current policy means the policy in the Casbin enforcer (aka in memory).
	e, err := casbin.NewEnforcer("examples/rbac_model.conf", "examples/rbac_policy.csv")
	s.Require().NoError(err)
	err = s.a.SavePolicy(e.GetModel())
	s.Require().NoError(err)

	s.e, err = casbin.NewEnforcer("examples/rbac_model.conf", s.a)
	s.Require().NoError(err)
}

func (s *AdapterTestSuite) TearDownTest() {
	err := s.a.Close()
	s.Require().NoError(err)
}

func (s *AdapterTestSuite) TestSaveLoad() {
	s.testGetPolicy([][]string{{"alice", "data1", "read"}, {"bob", "data2", "write"}, {"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}})
}

func (s *AdapterTestSuite) TestAutoSave() {
	// AutoSave is enabled by default.
	// Now we disable it.
	s.e.EnableAutoSave(false)

	// Because AutoSave is disabled, the policy change only affects the policy in Casbin enforcer,
	// it doesn't affect the policy in the storage.
	_, err := s.e.AddPolicy("alice", "data1", "write")
	s.Require().NoError(err)
	// Reload the policy from the storage to see the effect.
	err = s.e.LoadPolicy()
	s.Require().NoError(err)
	// This is still the original policy.
	s.testGetPolicy([][]string{{"alice", "data1", "read"}, {"bob", "data2", "write"}, {"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}})

	// Now we enable the AutoSave.
	s.e.EnableAutoSave(true)

	// Because AutoSave is enabled, the policy change not only affects the policy in Casbin enforcer,
	// but also affects the policy in the storage.
	_, err = s.e.AddPolicy("alice", "data1", "write")
	s.Require().NoError(err)
	// Reload the policy from the storage to see the effect.
	err = s.e.LoadPolicy()
	s.Require().NoError(err)
	// The policy has a new rule: {"alice", "data1", "write"}.
	s.testGetPolicy([][]string{{"alice", "data1", "read"}, {"bob", "data2", "write"}, {"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}, {"alice", "data1", "write"}})

	// Aditional AddPolicy have no effect
	_, err = s.e.AddPolicy("alice", "data1", "write")
	s.Require().NoError(err)
	err = s.e.LoadPolicy()
	s.Require().NoError(err)
	s.testGetPolicy([][]string{{"alice", "data1", "read"}, {"bob", "data2", "write"}, {"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}, {"alice", "data1", "write"}})
	s.Require().NoError(err)
}

func (s *AdapterTestSuite) TestConstructorOptions() {
	opts, err := pg.ParseURL(os.Getenv("PG_CONN"))
	s.Require().NoError(err)

	a, err := NewAdapter(opts)
	s.Require().NoError(err)
	defer a.Close()

	s.e, err = casbin.NewEnforcer("examples/rbac_model.conf", a)
	s.Require().NoError(err)

	s.testGetPolicy([][]string{{"alice", "data1", "read"}, {"bob", "data2", "write"}, {"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}})
}

func (s *AdapterTestSuite) TestRemovePolicy() {
	_, err := s.e.RemovePolicy("alice", "data1", "read")
	s.Require().NoError(err)

	s.testGetPolicy([][]string{{"bob", "data2", "write"}, {"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}})

	err = s.e.LoadPolicy()
	s.Require().NoError(err)

	s.testGetPolicy([][]string{{"bob", "data2", "write"}, {"data2_admin", "data2", "read"}, {"data2_admin", "data2", "write"}})
}

func (s *AdapterTestSuite) TestRemoveFilteredPolicy() {
	_, err := s.e.RemoveFilteredPolicy(0, "", "data2")
	s.Require().NoError(err)

	s.testGetPolicy([][]string{{"alice", "data1", "read"}})

	err = s.e.LoadPolicy()
	s.Require().NoError(err)

	s.testGetPolicy([][]string{{"alice", "data1", "read"}})
}

func TestAdapterTestSuite(t *testing.T) {
	suite.Run(t, new(AdapterTestSuite))
}