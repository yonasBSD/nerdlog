package main

type menuItem struct {
	Title   string
	Handler func(mv *MainView)
}

var mainMenu = []menuItem{
	{
		Title: "Back                 :back ",
		Handler: func(mv *MainView) {
			mv.params.OnCmd("back", CmdOpts{Internal: true})
		},
	},
	{
		Title: "Forward              :fwd  ",
		Handler: func(mv *MainView) {
			mv.params.OnCmd("fwd", CmdOpts{Internal: true})
		},
	},
	{
		Title: "Copy query command   :xclip",
		Handler: func(mv *MainView) {
			mv.params.OnCmd("xclip", CmdOpts{Internal: true})
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
