package ioc

import (
	ioc "github.com/sakuradon99/ioc/internal"
	"github.com/stretchr/testify/assert"
	"testing"
)

type stu struct {
	name string `property:"name"`
	age  int    `property:"age"`
}

type nestedStu struct {
	stu
	nestedStu stu `property:"nested_stu"`
}

type A struct {
	name       *string   `value:"name"`
	age        float32   `value:"age"`
	address    string    `value:"address;optional"`
	array      []string  `value:"array"`
	stu        stu       `value:"stu"`
	pointerStu *stu      `value:"pointer_stu"`
	arrayStu   []stu     `value:"array_stu"`
	nestedStu  nestedStu `value:"nested_stu"`
}

func (a A) Name() string {
	return "alice"
}

type B struct {
}

type I1 interface {
	Echo1() string
}

type C struct {
}

func (c *C) Echo1() string {
	return "c"
}

type D struct {
}

func (d *D) Echo1() string {
	return "d"
}

type I2 interface {
	Echo2() string
}

type E struct {
}

func (e *E) Echo2() string {
	return "e"
}

type F struct {
}

func (f *F) Echo2() string {
	return "f"
}

type App struct {
	a  *A `inject:""`
	b  *B `inject:";optional"`
	ic I1 `inject:"c"`
	id I1 `inject:"d"`
	i2 I2 `inject:""`
}

func (b *App) TestA() string {
	return b.a.Name()
}

func Test_IOC(t *testing.T) {
	t.Run("inject value", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		SetSourceFile("testdata/config.yaml")
		Register[A]()

		a, err := GetObject[A]("")
		assert.NoError(t, err)
		assert.Equal(t, "alice", *a.name)
		assert.Equal(t, float32(18), a.age)
		assert.Equal(t, "", a.address)
		assert.Equal(t, []string{"xxx", "ccc"}, a.array)
		assert.Equal(t, "bob", a.stu.name)
		assert.Equal(t, 20, a.stu.age)
		assert.Equal(t, "ppp", a.pointerStu.name)
		assert.Equal(t, 50, a.pointerStu.age)
		assert.Equal(t, "bbb", a.arrayStu[1].name)
		assert.Equal(t, 20, a.arrayStu[1].age)
		assert.Equal(t, "vvv", a.nestedStu.name)
		assert.Equal(t, 30, a.nestedStu.age)
		assert.Equal(t, "nnn", a.nestedStu.nestedStu.name)
		assert.Equal(t, 40, a.nestedStu.nestedStu.age)
	})

	t.Run("inject object", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		SetSourceFile("testdata/config.yaml")
		Register[A]()
		Register[C](Name("c"))
		Register[D](Name("d"))
		Register[E](Conditional("#use_e == true"))
		Register[F](Conditional("#use_e != true"))
		Register[App]()

		app, err := GetObject[App]("")
		if err != nil {
			panic(err)
		}
		assert.NoError(t, err)

		assert.Equal(t, "alice", app.TestA())
		assert.Equal(t, true, app.b == nil)
		assert.Equal(t, "c", app.ic.Echo1())
		assert.Equal(t, "d", app.id.Echo1())
		assert.Equal(t, "e", app.i2.Echo2())

		ic, err := GetInterface[I1]("c")
		assert.NoError(t, err)
		assert.Equal(t, "c", ic.Echo1())

		id, err := GetInterface[I1]("d")
		assert.NoError(t, err)
		assert.Equal(t, "d", id.Echo1())
	})

	t.Run("wrong source file", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		SetSourceFile("testdata/config1.yaml")
		Register[A]()

		_, err := GetObject[A]("")
		assert.Error(t, err)
	})
}
