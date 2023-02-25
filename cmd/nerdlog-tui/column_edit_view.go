package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/juju/errors"
	"github.com/rivo/tview"
)

type ColumnEditViewParams struct {
	// DoneFunc is called when the user submits the form. If it returns a non-nil
	// error, the form will show that error and will not be submitted.
	DoneFunc func(field SelectQueryField) error
}

type ColumnEditView struct {
	params   ColumnEditViewParams
	mainView *MainView

	field SelectQueryField

	flex *tview.Flex

	nameInput        *tview.InputField
	displayNameInput *tview.InputField
	stickyCheckbox   *tview.Checkbox
	frame            *tview.Frame
}

func NewColumnEditView(
	mainView *MainView, params *ColumnEditViewParams,
) *ColumnEditView {
	cev := &ColumnEditView{
		params:   *params,
		mainView: mainView,
	}

	var focusers []tview.Primitive
	getGenericTabHandler := func(curPrimitive tview.Primitive) func(event *tcell.EventKey) *tcell.EventKey {
		return func(event *tcell.EventKey) *tcell.EventKey {
			key := event.Key()

			nextIdx := 0
			prevIdx := 0

			for i, p := range focusers {
				if p != curPrimitive {
					continue
				}

				prevIdx = i - 1
				if prevIdx < 0 {
					prevIdx = len(focusers) - 1
				}

				nextIdx = i + 1
				if nextIdx >= len(focusers) {
					nextIdx = 0
				}
			}

			switch key {
			case tcell.KeyTab:
				cev.mainView.params.App.SetFocus(focusers[nextIdx])
				return nil

			case tcell.KeyBacktab:
				cev.mainView.params.App.SetFocus(focusers[prevIdx])
				return nil
			}

			return event
		}
	}

	cev.flex = tview.NewFlex().SetDirection(tview.FlexRow)

	nameLabel := tview.NewTextView()
	nameLabel.SetText("Name:")
	nameLabel.SetDynamicColors(true)
	cev.flex.AddItem(nameLabel, 1, 0, false)

	cev.nameInput = tview.NewInputField()
	cev.nameInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		event = cev.genericInputHandler(event, getGenericTabHandler(cev.nameInput), nil, nil)
		if event == nil {
			return nil
		}

		return event
	})
	cev.flex.AddItem(cev.nameInput, 1, 0, true)
	focusers = append(focusers, cev.nameInput)

	cev.flex.AddItem(nil, 1, 0, false)

	displayNameLabel := tview.NewTextView()
	displayNameLabel.SetText("Display name (empty means the same as Name):")
	displayNameLabel.SetDynamicColors(true)
	cev.flex.AddItem(displayNameLabel, 1, 0, false)

	cev.displayNameInput = tview.NewInputField()
	cev.displayNameInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		event = cev.genericInputHandler(event, getGenericTabHandler(cev.displayNameInput), nil, nil)
		if event == nil {
			return nil
		}

		return event
	})
	cev.flex.AddItem(cev.displayNameInput, 1, 0, true)
	focusers = append(focusers, cev.displayNameInput)

	cev.flex.AddItem(nil, 1, 0, false)

	stickyLabel := tview.NewTextView()
	stickyLabel.SetText("Sticky")
	stickyLabel.SetDynamicColors(true)

	cev.stickyCheckbox = tview.NewCheckbox()
	cev.stickyCheckbox.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		event = cev.genericInputHandler(event, getGenericTabHandler(cev.stickyCheckbox), nil, nil)
		if event == nil {
			return nil
		}

		return event
	})
	focusers = append(focusers, cev.stickyCheckbox)

	stickyFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
	stickyFlex.
		AddItem(tview.NewTextView().SetText("["), 1, 0, false).
		AddItem(cev.stickyCheckbox, 1, 0, false).
		AddItem(tview.NewTextView().SetText("]"), 1, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(stickyLabel, 0, 1, false).
		AddItem(nil, 0, 1, false)
	cev.flex.AddItem(stickyFlex, 1, 0, true)

	cev.flex.AddItem(nil, 0, 1, false)

	cev.frame = tview.NewFrame(cev.flex).SetBorders(0, 0, 0, 0, 0, 0)
	cev.frame.SetBorder(true).SetBorderPadding(0, 0, 1, 1)
	cev.frame.SetTitle("Edit column")

	return cev
}

func (cev *ColumnEditView) Show(field SelectQueryField) {
	cev.nameInput.SetText(field.Name)
	if field.DisplayName != field.Name {
		cev.displayNameInput.SetText(field.DisplayName)
	} else {
		cev.displayNameInput.SetText("")
	}
	cev.stickyCheckbox.SetChecked(field.Sticky)

	cev.mainView.showModal(
		pageNameColumnDetails, cev.frame,
		60,
		9,
		true,
	)
}

func (cev *ColumnEditView) Hide() {
	cev.mainView.hideModal(pageNameColumnDetails, true)
}

func (cev *ColumnEditView) GetSelectQueryField() SelectQueryField {
	field := SelectQueryField{
		Name:        cev.nameInput.GetText(),
		DisplayName: cev.displayNameInput.GetText(),
		Sticky:      cev.stickyCheckbox.IsChecked(),
	}

	if field.DisplayName == "" {
		field.DisplayName = field.Name
	}

	return field
}

func (cev *ColumnEditView) genericInputHandler(
	event *tcell.EventKey,
	genericTabHandler func(event *tcell.EventKey) *tcell.EventKey,
	getQFPart func(qf QueryFull) string,
	setQFPart func(qf *QueryFull, part string),
) *tcell.EventKey {
	event = genericTabHandler(event)
	if event == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyEnter:
		if err := cev.apply(); err != nil {
			cev.mainView.showMessagebox("err", "Error", err.Error(), nil)
		}
		return nil

	case tcell.KeyEsc:
		cev.Hide()
		return nil
	}

	return event
}

func (cev *ColumnEditView) apply() error {
	err := cev.params.DoneFunc(cev.GetSelectQueryField())
	if err != nil {
		return errors.Trace(err)
	}

	cev.Hide()

	return nil
}
