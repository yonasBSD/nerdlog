package main

type menuItem struct {
	Title   string
	Handler func(mv *MainView)
}

var mainMenu = []menuItem{
	{
		Title: "Back                 <Alt+Left> ",
		Handler: func(mv *MainView) {
			mv.params.OnCmd("back", CmdOpts{Internal: true})
		},
	},
	{
		Title: "Forward              <Alt+Right>",
		Handler: func(mv *MainView) {
			mv.params.OnCmd("fwd", CmdOpts{Internal: true})
		},
	},
	{
		Title: "Refresh              <F5>       ",
		Handler: func(mv *MainView) {
			mv.params.OnCmd("refresh", CmdOpts{Internal: true})
		},
	},
	{
		Title: "Hard refresh         <Shift+F5> ",
		Handler: func(mv *MainView) {
			mv.params.OnCmd("refresh!", CmdOpts{Internal: true})
		},
	},
	{
		Title: "Copy query command   :xclip     ",
		Handler: func(mv *MainView) {
			mv.params.OnCmd("xclip", CmdOpts{Internal: true})
		},
	},
	{
		Title: "Query debug info     :debug     ",
		Handler: func(mv *MainView) {
			mv.params.OnCmd("debug", CmdOpts{Internal: true})
		},
	},
	{
		Title: "About                :version   ",
		Handler: func(mv *MainView) {
			mv.params.OnCmd("version", CmdOpts{Internal: true})
		},
	},
}

func getMainMenuTitles() []string {
	ret := make([]string, 0, len(mainMenu))
	for _, item := range mainMenu {
		ret = append(ret, item.Title)
	}

	return ret
}
