package ioc

import (
	ioc "github.com/sakuradon99/ioc/internal"
	"github.com/stretchr/testify/assert"
	"testing"
)

type propertyStu struct {
	str       string    `property:"str"`
	strPtr    *string   `property:"str_ptr"`
	strOpt    *string   `property:"str_opt;optional"`
	int       int       `property:"int"`
	float     float32   `property:"float"`
	bool      bool      `property:"bool"`
	strArr    []string  `property:"string_arr"`
	strPtrArr []*string `property:"string_ptr_arr"`
}

type nestedStu struct {
	propertyStu
	str2 string `property:"str_2"`
}

type A struct {
	str         string      `value:"str"`
	strPtr      *string     `value:"str_ptr"`
	strOpt      *string     `value:"str_opt;optional"`
	int         int         `value:"int"`
	float       float32     `value:"float"`
	bool        bool        `value:"bool"`
	strArr      []string    `value:"string_arr"`
	strPtrArr   []*string   `value:"string_ptr_arr"`
	nextStr     string      `value:"next.str"`
	nextInt     int         `value:"next.int"`
	propertyStu propertyStu `value:"property_stu"`
	nestedStu   nestedStu   `value:"nested_stu"`
}

type B struct {
	a *A `inject:""`
}

type C struct {
	A
}

type D struct {
	c *C `inject:""`
}

type E struct {
	a *A `inject:""`
}

type F struct {
	b *B `inject:""`
	D
	params struct {
		e *E `inject:""`
	}
}

type I interface {
	Test() string
}

type Impl struct {
}

func (i *Impl) Test() string {
	return "test"
}

type IMulti interface {
	TestMulti() string
}

type ImplMulti1 struct {
}

func (i *ImplMulti1) TestMulti() string {
	return "test1"
}

type ImplMulti2 struct {
}

func (i *ImplMulti2) TestMulti() string {
	return "test2"
}

type G struct {
	a      *A
	i      I      `inject:""`
	multi1 IMulti `inject:"multi1"`
	multi2 IMulti `inject:"multi2"`

	str        string
	defaultStr string
}

func NewG(a *A, i I, params struct {
	multi1     IMulti `inject:"multi1"`
	multi2     IMulti `inject:"multi2"`
	defaultStr string
	str        string `value:"str"`
}) *G {
	return &G{
		a:          a,
		i:          i,
		multi1:     params.multi1,
		multi2:     params.multi2,
		str:        params.str,
		defaultStr: params.defaultStr,
	}
}

type H struct {
	i IMulti `inject:""`
}

type J struct {
	interfaceList []IMulti `inject:""`
}

func Test_IOC(t *testing.T) {
	t.Run("inject value", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		SetSourceFile("testdata/config.yaml")
		Register[A]()

		a, err := GetObject[A]("")
		assert.Nil(t, err)
		assert.NotNil(t, a)
		assert.Equal(t, "str", a.str)
		assert.Equal(t, "str_ptr", *a.strPtr)
		assert.Nil(t, a.strOpt)
		assert.Equal(t, 1, a.int)
		assert.Equal(t, float32(0.99), a.float)
		assert.True(t, a.bool)
		assert.Equal(t, []string{"str_1", "str_2"}, a.strArr)
		assert.Equal(t, 2, len(a.strPtrArr))
		assert.Equal(t, "str_ptr_1", *a.strPtrArr[0])
		assert.Equal(t, "str_ptr_2", *a.strPtrArr[1])
		assert.Equal(t, "next_str", a.nextStr)
		assert.Equal(t, 2, a.nextInt)

		assert.Equal(t, "str", a.propertyStu.str)
		assert.Equal(t, "str_ptr", *a.propertyStu.strPtr)
		assert.Nil(t, a.propertyStu.strOpt)
		assert.Equal(t, 1, a.propertyStu.int)
		assert.Equal(t, float32(0.99), a.propertyStu.float)
		assert.True(t, a.propertyStu.bool)
		assert.Equal(t, []string{"str_1", "str_2"}, a.propertyStu.strArr)
		assert.Equal(t, 2, len(a.propertyStu.strPtrArr))
		assert.Equal(t, "str_ptr_1", *a.propertyStu.strPtrArr[0])
		assert.Equal(t, "str_ptr_2", *a.propertyStu.strPtrArr[1])

		assert.Equal(t, "str", a.nestedStu.str)
		assert.Equal(t, "str_ptr", *a.nestedStu.strPtr)
		assert.Nil(t, a.nestedStu.strOpt)
		assert.Equal(t, 1, a.nestedStu.int)
		assert.Equal(t, float32(0.99), a.nestedStu.float)
		assert.True(t, a.nestedStu.bool)
		assert.Equal(t, []string{"str_1", "str_2"}, a.nestedStu.strArr)
		assert.Equal(t, 2, len(a.nestedStu.strPtrArr))
		assert.Equal(t, "str_ptr_1", *a.nestedStu.strPtrArr[0])
		assert.Equal(t, "str_ptr_2", *a.nestedStu.strPtrArr[1])
		assert.Equal(t, "str_2", a.nestedStu.str2)
	})

	t.Run("inject object", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		SetSourceFile("testdata/config.yaml")
		Register[A]()
		Register[B]()
		Register[C]()
		Register[D]()
		Register[E]()
		Register[F]()

		f, err := GetObject[F]("")
		assert.Nil(t, err)
		assert.NotNil(t, f)

		assert.Equal(t, "str", f.b.a.str)
		assert.Equal(t, "str", f.c.str)
		assert.Equal(t, "str", f.params.e.a.str)
	})

	t.Run("inject interface", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		Register[Impl]()
		Register[ImplMulti1](Name("multi1"))
		Register[ImplMulti2](Name("multi2"))
		Register[G]()

		g, err := GetObject[G]("")
		assert.Nil(t, err)
		assert.NotNil(t, g)

		assert.Equal(t, "test", g.i.Test())
		assert.Equal(t, "test1", g.multi1.TestMulti())
		assert.Equal(t, "test2", g.multi2.TestMulti())
	})

	t.Run("inject with constructor", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		SetSourceFile("testdata/config.yaml")
		Register[A]()
		Register[B]()
		Register[Impl]()
		Register[ImplMulti1](Name("multi1"))
		Register[ImplMulti2](Name("multi2"))
		Register[G](Constructor(NewG))

		g, err := GetObject[G]("")
		assert.Nil(t, err)
		assert.NotNil(t, g)

		assert.Equal(t, "str", g.a.str)
		assert.Equal(t, "test", g.i.Test())
		assert.Equal(t, "test1", g.multi1.TestMulti())
		assert.Equal(t, "test2", g.multi2.TestMulti())
		assert.Equal(t, "str", g.str)
		assert.Equal(t, "", g.defaultStr)
	})

	t.Run("inject with condition", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		SetSourceFile("testdata/config.yaml")
		Register[ImplMulti1](Conditional("#condition.use_impl_multi == 1"))
		Register[ImplMulti2](Conditional("#condition.use_impl_multi == 2"))
		Register[H]()

		h, err := GetObject[H]("")
		assert.Nil(t, err)
		assert.NotNil(t, h)

		assert.Equal(t, "test2", h.i.TestMulti())
	})

	t.Run("inject list of interface", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		Register[ImplMulti1]()
		Register[ImplMulti2]()
		Register[J]()

		j, err := GetObject[J]("")
		assert.Nil(t, err)
		assert.NotNil(t, j)

		assert.Equal(t, 2, len(j.interfaceList))
		arr := []string{j.interfaceList[0].TestMulti(), j.interfaceList[1].TestMulti()}
		assert.Contains(t, arr, "test1")
		assert.Contains(t, arr, "test2")
	})
}
