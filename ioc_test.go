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

type ObjectA struct {
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

type ObjectB struct {
	a *ObjectA `inject:""`
}

type ObjectC struct {
	ObjectA
}

type ObjectD struct {
	c *ObjectC `inject:""`
}

type ObjectE struct {
	a *ObjectA `inject:""`
}

type ObjectF struct {
	b *ObjectB `inject:""`
	ObjectD
	params struct {
		e *ObjectE `inject:""`
	}
}

type Interface interface {
	Test() string
}

type Impl struct {
}

func (i *Impl) Test() string {
	return "test"
}

type InterfaceMulti interface {
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

type ObjectG struct {
	a      *ObjectA
	b      *ObjectB       `inject:";optional"`
	i      Interface      `inject:""`
	multi1 InterfaceMulti `inject:"multi1"`
	multi2 InterfaceMulti `inject:"multi2"`

	str        string
	defaultStr string
}

func NewObjectG(a *ObjectA, i Interface, params struct {
	multi1     InterfaceMulti `inject:"multi1"`
	multi2     InterfaceMulti `inject:"multi2"`
	defaultStr string
	str        string `value:"str"`
}) *ObjectG {
	return &ObjectG{
		a:          a,
		i:          i,
		multi1:     params.multi1,
		multi2:     params.multi2,
		str:        params.str,
		defaultStr: params.defaultStr,
	}
}

type ObjectH struct {
	i InterfaceMulti `inject:""`
}

type ObjectJ struct {
	interfaceList []InterfaceMulti `inject:";optional"`
}

type ObjectK struct {
	aList []*ObjectA `inject:"a[12]"`
}

type ObjectL struct {
	iList []InterfaceMulti `inject:"*2"`
}

type ObjectM struct {
	a *ObjectA `inject:""`
}

func (o *ObjectM) Init() error {
	o.a.str = "init"
	return nil
}

type ObjectN struct {
	key1     string `value:"key1"`
	key2     string `value:"key2"`
	nextKey1 string `value:"next.key1"`
}

type ObjectO struct {
	i Interface `inject:"alisaI"`
}

type ObjectP struct {
	iMap map[string]InterfaceMulti `inject:"*"`
}

func Test_IOC_success(t *testing.T) {
	t.Run("inject value", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		_ = AddValueProvider(NewFileValueProvider("testdata/config.yaml"))
		Register[ObjectA]()

		a, err := GetObject[ObjectA]("")
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
		_ = AddValueProvider(NewFileValueProvider("testdata/config.yaml"))
		Register[ObjectA]()
		Register[ObjectB]()
		Register[ObjectC]()
		Register[ObjectD]()
		Register[ObjectE]()
		Register[ObjectF]()

		f, err := GetObject[ObjectF]("")
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
		Register[ObjectG]()

		g, err := GetObject[ObjectG]("")
		assert.Nil(t, err)
		assert.NotNil(t, g)

		assert.Nil(t, g.b)
		assert.Equal(t, "test", g.i.Test())
		assert.Equal(t, "test1", g.multi1.TestMulti())
		assert.Equal(t, "test2", g.multi2.TestMulti())
	})

	t.Run("inject with constructor", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		_ = AddValueProvider(NewFileValueProvider("testdata/config.yaml"))
		Register[ObjectA](Optional())
		Register[ObjectB]()
		Register[Impl]()
		Register[ImplMulti1](Name("multi1"))
		Register[ImplMulti2](Name("multi2"))
		Register[ObjectG](Constructor(NewObjectG))

		g, err := GetObject[ObjectG]("")
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
		_ = AddValueProvider(NewFileValueProvider("testdata/config.yaml"))
		Register[ImplMulti1](Conditional("#condition.use_impl_multi == 1"))
		Register[ImplMulti2](Conditional("#condition.use_impl_multi == 2"))
		Register[ObjectH]()

		h, err := GetObject[ObjectH]("")
		assert.Nil(t, err)
		assert.NotNil(t, h)

		assert.Equal(t, "test2", h.i.TestMulti())
	})

	t.Run("inject list of interface", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		Register[ImplMulti1]()
		Register[ImplMulti2]()
		Register[ObjectJ]()

		j, err := GetObject[ObjectJ]("")
		assert.Nil(t, err)
		assert.NotNil(t, j)

		assert.Equal(t, 2, len(j.interfaceList))
		arr := []string{j.interfaceList[0].TestMulti(), j.interfaceList[1].TestMulti()}
		assert.Contains(t, arr, "test1")
		assert.Contains(t, arr, "test2")
	})

	t.Run("inject list of object", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		_ = AddValueProvider(NewFileValueProvider("testdata/config.yaml"))
		Register[ObjectA](Name("a1"))
		Register[ObjectA](Name("a2"))
		Register[ObjectA](Name("a3"))
		Register[ObjectK]()
		k, err := GetObject[ObjectK]("")
		assert.Nil(t, err)
		assert.NotNil(t, k)

		assert.Equal(t, 2, len(k.aList))
		assert.Equal(t, "str", k.aList[0].str)
		assert.Equal(t, "str", k.aList[1].str)
	})

	t.Run("inject list of interface with name expr", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		Register[ImplMulti1](Name("multi1"))
		Register[ImplMulti2](Name("multi2"))
		Register[ObjectL]()

		l, err := GetObject[ObjectL]("")
		assert.Nil(t, err)
		assert.NotNil(t, l)

		assert.Equal(t, 1, len(l.iList))
		assert.Equal(t, "test2", l.iList[0].TestMulti())
	})

	t.Run("get interface", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		Register[ImplMulti1](Name("multi1"))
		Register[ImplMulti2](Name("multi2"))

		i, err := GetInterface[InterfaceMulti]("multi1")
		assert.Nil(t, err)
		assert.NotNil(t, i)
		assert.Equal(t, "test1", i.TestMulti())

		i, err = GetInterface[InterfaceMulti]("multi2")
		assert.Nil(t, err)
		assert.NotNil(t, i)
		assert.Equal(t, "test2", i.TestMulti())
	})

	t.Run("get object list", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		_ = AddValueProvider(NewFileValueProvider("testdata/config.yaml"))
		Register[ObjectA](Name("a1"))
		Register[ObjectA](Name("a2"))
		Register[ObjectA](Name("a3"))

		objects, err := GetObjectList[ObjectA]("a[23]")
		assert.Nil(t, err)
		assert.NotNil(t, objects)

		assert.Equal(t, 2, len(objects))
		assert.Equal(t, "str", objects[0].str)
		assert.Equal(t, "str", objects[1].str)
	})

	t.Run("get interface list", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		Register[ImplMulti1](Name("multi1"))
		Register[ImplMulti2](Name("multi2"))
		Register[ImplMulti2](Name("multi2-2"))

		interfaces, err := GetInterfaceList[InterfaceMulti]("multi2*")
		assert.Nil(t, err)
		assert.NotNil(t, interfaces)

		assert.Equal(t, 2, len(interfaces))
		assert.Equal(t, "test2", interfaces[0].TestMulti())
		assert.Equal(t, "test2", interfaces[1].TestMulti())
	})

	t.Run("get object map", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		_ = AddValueProvider(NewFileValueProvider("testdata/config.yaml"))
		Register[ObjectA](Name("a1"))
		Register[ObjectA](Name("a2"))
		Register[ObjectA](Name("a3"))

		nameToObject, err := GetObjectMap[ObjectA]("a[23]")
		assert.Nil(t, err)
		assert.NotNil(t, nameToObject)

		assert.Equal(t, 2, len(nameToObject))
		assert.Equal(t, "str", nameToObject["a2"].str)
		assert.Equal(t, "str", nameToObject["a3"].str)
	})

	t.Run("process object initializing", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		_ = AddValueProvider(NewFileValueProvider("testdata/config.yaml"))
		Register[ObjectA]()
		Register[ObjectM]()

		m, err := GetObject[ObjectM]("")
		assert.Nil(t, err)
		assert.NotNil(t, m)

		assert.Equal(t, "init", m.a.str)
		assert.Equal(t, "str_ptr", *m.a.strPtr)
	})

	t.Run("inject value with map value provider", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		_ = AddValueProvider(NewMapValueProvider(map[string]any{
			"key1": "value1",
			"key2": "value2",
			"next": map[string]any{
				"key1": "next_value1",
			},
		}))
		Register[ObjectN]()

		n, err := GetObject[ObjectN]("")
		assert.Nil(t, err)
		assert.NotNil(t, n)

		assert.Equal(t, "value1", n.key1)
		assert.Equal(t, "value2", n.key2)
		assert.Equal(t, "next_value1", n.nextKey1)
	})

	t.Run("inject object with alisa", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		Register[Impl](Alias("alisaI"))
		Register[ObjectO]()

		o, err := GetObject[ObjectO]("")
		assert.Nil(t, err)
		assert.NotNil(t, o)

		assert.NotNil(t, o.i)
		assert.Equal(t, "test", o.i.Test())
	})

	t.Run("inject object with map", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		Register[ImplMulti1](Name("multi1"))
		Register[ImplMulti2](Name("multi2"))
		Register[ObjectP]()

		p, err := GetObject[ObjectP]("")
		assert.Nil(t, err)
		assert.NotNil(t, p)

		assert.Equal(t, 2, len(p.iMap))
		assert.NotNil(t, p.iMap["multi1"])
		assert.NotNil(t, p.iMap["multi2"])
		assert.Equal(t, "test1", p.iMap["multi1"].TestMulti())
		assert.Equal(t, "test2", p.iMap["multi2"].TestMulti())
	})

	t.Run("get interface map", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		Register[ImplMulti1](Name("multi1"))
		Register[ImplMulti2](Name("multi2"))
		nameToInterface, err := GetInterfaceMap[InterfaceMulti]("*")

		assert.Nil(t, err)
		assert.NotNil(t, nameToInterface)

		assert.Equal(t, 2, len(nameToInterface))
		assert.NotNil(t, nameToInterface["multi1"])
		assert.NotNil(t, nameToInterface["multi2"])
		assert.Equal(t, "test1", nameToInterface["multi1"].TestMulti())
		assert.Equal(t, "test2", nameToInterface["multi2"].TestMulti())
	})

	t.Run("get value", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		_ = AddValueProvider(NewFileValueProvider("testdata/config.yaml"))

		v, ok, err := GetValue[string]("str")

		assert.Nil(t, err)
		assert.True(t, ok)
		assert.Equal(t, "str", v)
	})
}
