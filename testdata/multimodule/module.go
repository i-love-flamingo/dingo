package multimodule

import "flamingo.me/dingo"

type MainSvc interface{ Main() string }
type ScopeSvc interface{ Scope() string }
type SSRSvc interface{ SSR() string }

type mainImpl struct{}

func (mainImpl) Main() string { return "main" }

type scopeImpl struct{}

func (scopeImpl) Scope() string { return "scope" }

type ssrImpl struct{}

func (ssrImpl) SSR() string { return "ssr" }

type Module struct{}

func (*Module) Configure(i *dingo.Injector) { i.Bind((*MainSvc)(nil)).To(mainImpl{}) }

type ScopeModule struct{}

func (*ScopeModule) Configure(i *dingo.Injector) { i.Bind((*ScopeSvc)(nil)).To(scopeImpl{}) }

type SSRModule struct{}

func (*SSRModule) Configure(i *dingo.Injector) { i.Bind((*SSRSvc)(nil)).To(ssrImpl{}) }
