import utils from "utils";

const mcpNamedKeys = {
  enter: { key: "Enter", keyCode: 13, code: "Enter" },
  escape: { key: "Escape", keyCode: 27, code: "Escape" },
  space: { key: " ", keyCode: 32, code: "Space" },
  tab: { key: "Tab", keyCode: 9, code: "Tab" },
  backspace: { key: "Backspace", keyCode: 8, code: "Backspace" },
  delete: { key: "Delete", keyCode: 46, code: "Delete" },
  insert: { key: "Insert", keyCode: 45, code: "Insert" },
  up: { key: "ArrowUp", keyCode: 38, code: "ArrowUp" },
  down: { key: "ArrowDown", keyCode: 40, code: "ArrowDown" },
  left: { key: "ArrowLeft", keyCode: 37, code: "ArrowLeft" },
  right: { key: "ArrowRight", keyCode: 39, code: "ArrowRight" },
  home: { key: "Home", keyCode: 36, code: "Home" },
  end: { key: "End", keyCode: 35, code: "End" },
  pgup: { key: "PageUp", keyCode: 33, code: "PageUp" },
  pgdn: { key: "PageDown", keyCode: 34, code: "PageDown" },
};

for (let i = 1; i <= 24; i++) {
  mcpNamedKeys[`f${i}`] = {
    key: `F${i}`,
    keyCode: 111 + i,
    code: `F${i}`,
  };
}

export default (MixinBase) =>
  class extends MixinBase {
    _handleBackgroundMessage(message) {
      let input, url, config;
      const parts = message.split(",");
      const command = parts[0];
      switch (command) {
        case "/config":
          config = JSON.parse(utils.rebuildArgsToSingleArg(parts));
          this._loadConfig(config);
          break;
        case "/request_raw_text":
          if (parts[1]) {
            this._raw_mode_type = "raw_text_" + parts[1].toLowerCase();
          } else if (!this._raw_mode_type) {
            this._raw_mode_type = "raw_text_plain";
          }
          this.sendRawText();
          break;
        case "/page_metadata":
          this._updatePageMetadata(JSON.parse(utils.rebuildArgsToSingleArg(parts)));
          break;
        case "/request_frame":
          this.sendFrame();
          break;
        case "/rebuild_text":
          if (this._is_interactive_mode) {
            this.sendAllBigFrames();
          }
          break;
        case "/scroll_status":
          this._handleScroll(parts[1], parts[2]);
          break;
        case "/tty_size":
          this._handleTTYSize(parts[1], parts[2]);
          break;
        case "/stdin":
          input = JSON.parse(utils.rebuildArgsToSingleArg(parts));
          this._handleUserInput(input);
          break;
        case "/input_box":
          input = JSON.parse(utils.rebuildArgsToSingleArg(parts));
          this._handleInputBoxContent(input);
          break;
        case "/url":
          url = utils.rebuildArgsToSingleArg(parts);
          document.location.href = url;
          break;
        case "/history_back":
          history.go(-1);
          break;
        case "/window_stop":
          window.stop();
          break;
        case "/mcp_action":
          this._handleMCPAction(utils.rebuildArgsToSingleArg(parts));
          break;
        default:
          this.log("Unknown command sent to tab", message);
      }
    }

    _launch() {
      const mode = this.config.http_server_mode_type;
      if (mode.includes("raw_text_")) {
        this._is_raw_text_mode = true;
        this._is_interactive_mode = false;
        this._raw_mode_type = mode;
        this.sendRawText();
      }
      if (mode === "interactive") {
        this._is_raw_text_mode = false;
        this._is_interactive_mode = true;
        this._setupInteractiveMode();
      }
    }

    _loadConfig(config) {
      this.config = config;
      this._postSetupConstructor();
      this._launch();
    }

    _handleUserInput(input) {
      this._handleSpecialKeys(input);
      this._handleCharBasedKeys(input);
      this._handleMouse(input);
    }

    _handleSpecialKeys(input) {
      let state, message;
      switch (input.key) {
        case 18: // CTRL+r
          window.location.reload();
          break;
        case 284: // F6
          state = this.config.browsh.use_experimental_text_visibility;
          state = !state;
          this.config.browsh.use_experimental_text_visibility = state;
          message = state ? "on" : "off";
          this.sendMessage(
            `/status,info,Experimental text visibility: ${message}`
          );
          this.sendSmallTextFrame();
          break;
      }
    }

    _handleCharBasedKeys(input) {
      switch (input.char) {
        default:
          this._triggerKeyPress(input);
      }
    }

    _handleInputBoxContent(input) {
      let input_box = document.querySelectorAll(
        `[data-browsh-id="${input.id}"]`
      )[0];
      if (input_box) {
        input_box.focus();
        if (input_box.getAttribute("role") == "textbox") {
          input_box.innerHTML = input.text;
        } else {
          input_box.value = input.text;
        }
      }
    }

    // TODO: Dragndrop doesn't seem to work :/
    _handleMouse(input) {
      this._rememberMousePositionFromRaw(input.mouse_x, input.mouse_y);
      switch (input.button) {
        case 1:
          this._mouseAction("mousemove", input.mouse_x, input.mouse_y);
          if (!this._mousedown) {
            this._mouseAction("mousedown", input.mouse_x, input.mouse_y);
            setTimeout(() => {
              this.sendSmallTextFrame();
            }, 500);
          }
          this._mousedown = true;
          break;
        case 0:
          this._mouseAction("mousemove", input.mouse_x, input.mouse_y);
          if (this._mousedown) {
            this._mouseAction("click", input.mouse_x, input.mouse_y);
            this._mouseAction("mouseup", input.mouse_x, input.mouse_y);
          }
          this._mousedown = false;
          break;
      }
    }

    _handleTTYSize(x, y) {
      if (!this._is_first_frame_finished) {
        this.dimensions.tty.width = parseInt(x);
        this.dimensions.tty.height = parseInt(y);
        this.dimensions.update();
        this.sendAllBigFrames();
      }
    }

    _handleScroll(x, y) {
      this.dimensions.frame.x_scroll = parseInt(x);
      this.dimensions.frame.y_scroll = parseInt(y);
      this.dimensions.update();
      window.scrollTo(
        this.dimensions.frame.x_scroll / this.dimensions.scale_factor.width,
        this.dimensions.frame.y_scroll / this.dimensions.scale_factor.height
      );
      this._mightSendBigFrames();
    }

    _handleMCPAction(actionArg) {
      let action;
      try {
        action = JSON.parse(actionArg);
      } catch (_error) {
        action = {
          name: actionArg,
        };
      }
      switch (action.name) {
        case "pagedown":
        case "pageup":
        case "home":
        case "end":
          this._handleMCPScrollAction(action.name);
          break;
        case "backward":
        case "forward":
          this._handleMCPNavigationAction(action.name);
          break;
        case "mouse":
          this._handleMCPMouseAction(action);
          break;
        case "keyboard_input":
          this._handleMCPKeyboardInput(action);
          break;
        default:
          this.log("Unknown MCP action sent to tab", actionArg);
      }
    }

    _handleMCPScrollAction(actionName) {
      if (actionName === "pagedown") {
        window.scrollBy(0, window.innerHeight);
      } else if (actionName === "pageup") {
        window.scrollBy(0, -window.innerHeight);
      } else if (actionName === "home") {
        window.scrollTo(0, 0);
      } else if (actionName === "end") {
        window.scrollTo(0, this._mcpPageScrollHeight());
      }
    }

    _handleMCPNavigationAction(actionName) {
      if (actionName === "backward") {
        history.go(-1);
      } else if (actionName === "forward") {
        history.go(1);
      }
    }

    _handleMCPMouseAction(action) {
      if (!action.request_id) {
        return;
      }
      const target = this._mcpResolveMouseTarget(action.coordinates);
      if (!target || !target.element) {
        this._sendMCPActionResult(
          action.request_id,
          "",
          "Could not resolve mouse target on the current page."
        );
        return;
      }
      this._rememberMousePositionFromDOM(target.domX, target.domY);
      this._dispatchMCPMouseEvent(target, "mousemove", action.action);
      if (action.action === "move") {
        this._sendMCPActionResult(action.request_id, "", "");
        return;
      }
      target.element.focus();
      if (action.action === "click") {
        this._dispatchMCPMouseEvent(target, "mousedown", action.action);
        this._dispatchMCPMouseEvent(target, "mouseup", action.action);
        this._mcpActivateElement(target);
      } else if (action.action === "right_click") {
        this._dispatchMCPMouseEvent(target, "mousedown", action.action);
        this._dispatchMCPMouseEvent(target, "mouseup", action.action);
        this._dispatchMCPMouseEvent(target, "contextmenu", action.action);
      } else {
        this._sendMCPActionResult(
          action.request_id,
          "",
          `Unsupported mouse action '${action.action}'.`
        );
        return;
      }
      this._sendMCPActionResult(
        action.request_id,
        this._mcpFocusedCursorText(),
        ""
      );
    }

    _handleMCPKeyboardInput(action) {
      if (typeof action.text === "string" && action.text.length > 0) {
        this._applyMCPTextInput(action.text);
      }
      if (Array.isArray(action.keys)) {
        action.keys.forEach((keySpec) => {
          const keyObject = this._buildMCPKeyObject(keySpec);
          if (!keyObject) {
            this.log(`Unsupported MCP key: ${keySpec}`);
            return;
          }
          this._dispatchMCPKeyPress(keyObject);
        });
      }
    }

    _applyMCPTextInput(text) {
      if (this._insertTextIntoActiveElement(text)) {
        return;
      }
      Array.from(text).forEach((character) => {
        const keyObject = this._buildMCPKeyObject(character);
        if (keyObject) {
          this._dispatchMCPKeyPress(keyObject);
        }
      });
    }

    _insertTextIntoActiveElement(text) {
      const element = this._mcpEventTarget();
      if (!this._isMCPTextEditableElement(element)) {
        return false;
      }
      element.focus();
      if (element.isContentEditable || element.getAttribute("role") == "textbox") {
        return this._insertTextIntoMCPContentEditable(element, text);
      }
      return this._insertTextIntoMCPFormField(element, text);
    }

    _insertTextIntoMCPFormField(element, text) {
      const start =
        typeof element.selectionStart === "number"
          ? element.selectionStart
          : element.value.length;
      const end =
        typeof element.selectionEnd === "number"
          ? element.selectionEnd
          : element.value.length;
      if (typeof element.setRangeText === "function") {
        element.setRangeText(text, start, end, "end");
      } else {
        element.value =
          element.value.slice(0, start) + text + element.value.slice(end);
        if (typeof element.setSelectionRange === "function") {
          const position = start + text.length;
          element.setSelectionRange(position, position);
        }
      }
      this._dispatchMCPInputEvent(element, text);
      return true;
    }

    _insertTextIntoMCPContentEditable(element, text) {
      element.focus();
      if (document.queryCommandSupported && document.queryCommandSupported("insertText")) {
        if (document.execCommand("insertText", false, text)) {
          this._dispatchMCPInputEvent(element, text);
          return true;
        }
      }
      const selection = window.getSelection();
      if (!selection) {
        return false;
      }
      if (!selection.rangeCount) {
        const range = document.createRange();
        range.selectNodeContents(element);
        range.collapse(false);
        selection.removeAllRanges();
        selection.addRange(range);
      }
      const range = selection.getRangeAt(0);
      range.deleteContents();
      const textNode = document.createTextNode(text);
      range.insertNode(textNode);
      range.setStartAfter(textNode);
      range.collapse(true);
      selection.removeAllRanges();
      selection.addRange(range);
      this._dispatchMCPInputEvent(element, text);
      return true;
    }

    _dispatchMCPInputEvent(element, text) {
      let event;
      try {
        event = new InputEvent("input", {
          bubbles: true,
          data: text,
          inputType: "insertText",
        });
      } catch (_error) {
        event = new Event("input", {
          bubbles: true,
        });
      }
      element.dispatchEvent(event);
    }

    _dispatchMCPKeyPress(keyObject) {
      const element = this._mcpEventTarget();
      if (
        this._isMCPTextEditableElement(element) &&
        !keyObject.ctrlKey &&
        !keyObject.altKey &&
        this._isMCPPrintableKey(keyObject)
      ) {
        this._insertTextIntoActiveElement(keyObject.key);
        return;
      }
      const keyEventConfig = {
        bubbles: true,
        cancelable: true,
        composed: true,
        key: keyObject.key,
        code: keyObject.code,
        keyCode: keyObject.keyCode,
        which: keyObject.keyCode,
        charCode: this._isMCPPrintableKey(keyObject) ? keyObject.key.charCodeAt(0) : 0,
        shiftKey: !!keyObject.shiftKey,
        ctrlKey: !!keyObject.ctrlKey,
        altKey: !!keyObject.altKey,
      };
      const eventDown = new KeyboardEvent("keydown", keyEventConfig);
      const eventUp = new KeyboardEvent("keyup", keyEventConfig);
      const shouldApplyDefault = element.dispatchEvent(eventDown);
      if (this._shouldDispatchMCPKeyPressEvent(keyObject, element)) {
        const eventPress = new KeyboardEvent("keypress", keyEventConfig);
        element.dispatchEvent(eventPress);
      }
      if (shouldApplyDefault) {
        this._applyMCPDefaultKeyAction(keyObject, element);
      }
      element.dispatchEvent(eventUp);
    }

    _shouldDispatchMCPKeyPressEvent(keyObject, element) {
      if (keyObject.keyCode === 13 && element.tagName === "INPUT") {
        return true;
      }
      return (
        this._isMCPPrintableKey(keyObject) &&
        !keyObject.ctrlKey &&
        !keyObject.altKey
      );
    }

    _applyMCPDefaultKeyAction(keyObject, element) {
      if (this._isMCPTextEditableElement(element)) {
        return;
      }
      switch (keyObject.key) {
        case "PageDown":
          window.scrollBy(0, window.innerHeight);
          break;
        case "PageUp":
          window.scrollBy(0, -window.innerHeight);
          break;
        case "Home":
          window.scrollTo(0, 0);
          break;
        case "End":
          window.scrollTo(0, this._mcpPageScrollHeight());
          break;
        case "ArrowUp":
          window.scrollBy(0, -40);
          break;
        case "ArrowDown":
          window.scrollBy(0, 40);
          break;
        case "ArrowLeft":
          window.scrollBy(-40, 0);
          break;
        case "ArrowRight":
          window.scrollBy(40, 0);
          break;
        case " ":
          window.scrollBy(0, keyObject.shiftKey ? -window.innerHeight : window.innerHeight);
          break;
      }
    }

    _buildMCPKeyObject(keySpec) {
      if (typeof keySpec !== "string") {
        return null;
      }
      const modifiers = {
        shiftKey: false,
        ctrlKey: false,
        altKey: false,
      };
      let baseKey = null;
      keySpec
        .split("+")
        .map((part) => part.trim())
        .filter(Boolean)
        .forEach((part) => {
          const lowerPart = part.toLowerCase();
          if (lowerPart === "shift") {
            modifiers.shiftKey = true;
            return;
          }
          if (lowerPart === "ctrl") {
            modifiers.ctrlKey = true;
            return;
          }
          if (lowerPart === "alt") {
            modifiers.altKey = true;
            return;
          }
          if (baseKey === null) {
            baseKey = part;
          } else {
            baseKey = "";
          }
        });
      if (!baseKey) {
        return null;
      }
      const namedKey = mcpNamedKeys[baseKey.toLowerCase()];
      if (namedKey) {
        return Object.assign({}, namedKey, modifiers);
      }
      if (baseKey.length !== 1) {
        return null;
      }
      const upperKey = baseKey.toUpperCase();
      let code = "Unidentified";
      if (/^[A-Z]$/.test(upperKey)) {
        code = `Key${upperKey}`;
      } else if (/^[0-9]$/.test(baseKey)) {
        code = `Digit${baseKey}`;
      }
      return {
        key:
          modifiers.shiftKey && /^[a-z]$/i.test(baseKey)
            ? upperKey
            : baseKey,
        keyCode: upperKey.charCodeAt(0),
        code,
        shiftKey: modifiers.shiftKey,
        ctrlKey: modifiers.ctrlKey,
        altKey: modifiers.altKey,
      };
    }

    _isMCPPrintableKey(keyObject) {
      return keyObject.key.length === 1;
    }

    _mcpEventTarget() {
      return document.activeElement || document.body || document.documentElement;
    }

    _isMCPTextEditableElement(element) {
      if (!element || element.disabled || element.readOnly) {
        return false;
      }
      if (element.isContentEditable || element.getAttribute("role") == "textbox") {
        return true;
      }
      if (element.tagName === "TEXTAREA") {
        return true;
      }
      if (element.tagName !== "INPUT") {
        return false;
      }
      return ![
        "button",
        "checkbox",
        "color",
        "date",
        "datetime-local",
        "file",
        "hidden",
        "image",
        "month",
        "radio",
        "range",
        "reset",
        "submit",
        "time",
        "week",
      ].includes((element.type || "text").toLowerCase());
    }

    _mcpPageScrollHeight() {
      return Math.max(
        document.body ? document.body.scrollHeight : 0,
        document.documentElement ? document.documentElement.scrollHeight : 0
      );
    }

    _mcpResolveMouseTarget(coordinates) {
      if (
        !coordinates ||
        typeof coordinates.x !== "number" ||
        typeof coordinates.y !== "number"
      ) {
        return null;
      }
      const index = coordinates.y * this.dimensions.frame.width + coordinates.x;
      const cell = this.text_builder.tty_grid.cells[index];
      const [domX, domY] = this._getDOMCoordsFromMouseCoords(
        coordinates.x,
        coordinates.y
      );
      window.scrollTo(
        Math.max(0, domX - window.innerWidth / 2),
        Math.max(0, domY - window.innerHeight / 2)
      );
      const viewportX = domX - window.scrollX;
      const viewportY = domY - window.scrollY;
      const pointedElement = document.elementFromPoint(viewportX, viewportY);
      let element = pointedElement;
      if (cell && cell.parent_element) {
        element =
          this._mcpActionableElement(cell.parent_element) ||
          this._mcpActionableElement(pointedElement) ||
          cell.parent_element;
      } else {
        element = this._mcpActionableElement(pointedElement) || pointedElement;
      }
      if (!element) {
        return null;
      }
      return {
        domX,
        domY,
        element,
        viewportX,
        viewportY,
      };
    }

    _updatePageMetadata(metadata) {
      if (!this._runtime_state) {
        return;
      }
      this._runtime_state.security_status_text =
        (metadata && metadata.security_status_text) || "";
    }

    _rememberMousePositionFromRaw(x, y) {
      const [domX, domY] = this._getDOMCoordsFromMouseCoords(x, y);
      this._rememberMousePositionFromDOM(domX, domY);
    }

    _rememberMousePositionFromDOM(domX, domY) {
      if (
        !this._runtime_state ||
        typeof domX !== "number" ||
        typeof domY !== "number"
      ) {
        return;
      }
      this._runtime_state.last_mouse_position = {
        x: domX,
        y: domY,
      };
    }

    _mcpActionableElement(element) {
      if (!element) {
        return null;
      }
      if (typeof element.closest === "function") {
        return element.closest(
          "a,button,input,textarea,select,option,[role='button'],[role='link'],[role='textbox']"
        );
      }
      return element;
    }

    _mcpActivateElement(target) {
      if (typeof target.element.click === "function") {
        target.element.click();
        return;
      }
      this._dispatchMCPMouseEvent(target, "click", "click");
    }

    _dispatchMCPMouseEvent(target, actionName, mouseAction) {
      let button = 0;
      if (mouseAction === "right_click") {
        button = 2;
      }
      var mouseEvent = document.createEvent("MouseEvents");
      mouseEvent.initMouseEvent(
        actionName,
        true,
        true,
        window,
        0,
        0,
        0,
        target.viewportX,
        target.viewportY,
        false,
        false,
        false,
        false,
        button,
        null
      );
      target.element.dispatchEvent(mouseEvent);
    }

    _sendMCPActionResult(requestId, cursorText, error) {
      const payload = {
        request_id: requestId,
      };
      if (cursorText) {
        payload.cursor_text = cursorText;
      }
      if (error) {
        payload.error = error;
      }
      this.sendMessage(`/mcp_action_result,${JSON.stringify(payload)}`);
    }

    _mcpFocusedCursorText() {
      const element = this._mcpEventTarget();
      if (!this._isMCPTextEditableElement(element)) {
        return "";
      }
      if (element.tagName === "INPUT" || element.tagName === "TEXTAREA") {
        if (
          typeof element.selectionStart !== "number" ||
          typeof element.selectionEnd !== "number" ||
          element.selectionStart !== element.selectionEnd
        ) {
          return "";
        }
        return this._mcpTextWithCursor(
          element.value || "",
          element.selectionStart
        );
      }
      if (element.isContentEditable || element.getAttribute("role") == "textbox") {
        const selection = window.getSelection();
        if (
          !selection ||
          !selection.rangeCount ||
          !selection.isCollapsed ||
          !element.contains(selection.anchorNode)
        ) {
          return "";
        }
        const range = selection.getRangeAt(0).cloneRange();
        range.selectNodeContents(element);
        range.setEnd(selection.anchorNode, selection.anchorOffset);
        return this._mcpTextWithCursor(
          element.textContent || "",
          range.toString().length
        );
      }
      return "";
    }

    _mcpTextWithCursor(text, cursorIndex) {
      return text.slice(0, cursorIndex) + "|" + text.slice(cursorIndex);
    }

    _triggerKeyPress(key) {
      let el = document.activeElement;
      if (el == null) {
        this.log(
          `Not pressing '${key.char}(${key.key})' as there is no active element`
        );
        return;
      }
      const key_object = {
        key: key.char,
        keyCode: key.key,
      };
      let event_press = new KeyboardEvent("keypress", key_object);
      let event_down = new KeyboardEvent("keydown", key_object);
      let event_up = new KeyboardEvent("keyup", key_object);
      // Generally sending down/up serves more use cases. But default input forms
      // don't listen for down/up to make the form submit. So this makes the assumption
      // that it's okay to send ENTER twice to an input box without any serious side
      // effects.
      if (key.key === 13 && el.tagName === "INPUT") {
        el.dispatchEvent(event_press);
      } else {
        el.dispatchEvent(event_down);
        el.dispatchEvent(event_up);
      }
    }

    _mouseAction(type, x, y) {
      const [dom_x, dom_y] = this._getDOMCoordsFromMouseCoords(x, y);
      const element = document.elementFromPoint(
        dom_x - window.scrollX,
        dom_y - window.scrollY
      );
      element.focus();
      var clickEvent = document.createEvent("MouseEvents");
      clickEvent.initMouseEvent(
        type,
        true,
        true,
        window,
        0,
        0,
        0,
        dom_x,
        dom_y,
        false,
        false,
        false,
        false,
        0,
        null
      );
      element.dispatchEvent(clickEvent);
    }

    // The user clicks on a TTY grid which has a significantly lower resolution than the
    // actual browser window. So we scale the coordinates up as if the user clicked on the
    // the central "pixel" of a TTY cell.
    //
    // Furthermore if the TTY click is on a readable character then the click is proxied
    // to the original position of the character before TextBuilder snapped the character into
    // position.
    _getDOMCoordsFromMouseCoords(x, y) {
      let dom_x, dom_y, char, original_position;
      const index = y * this.dimensions.frame.width + x;
      if (this.text_builder.tty_grid.cells[index] !== undefined) {
        char = this.text_builder.tty_grid.cells[index].rune;
      } else {
        char = false;
      }
      if (!char || char === "▄") {
        dom_x = x * this.dimensions.char.width;
        dom_y = y * this.dimensions.char.height;
      } else {
        // Recall that text can be shifted from its original position in the browser in order
        // to snap it consistently to the TTY grid.
        original_position = this.text_builder.tty_grid.cells[index].dom_coords;
        dom_x = original_position.x;
        dom_y = original_position.y;
      }
      return [
        dom_x + this.dimensions.char.width / 2,
        dom_y + this.dimensions.char.height / 2,
      ];
    }

    _sendTabInfo() {
      const title_object = document.getElementsByTagName("title");
      let info = {
        url: document.location.href,
        title: title_object.length ? title_object[0].innerHTML : "",
      };
      this.sendMessage(`/tab_info,${JSON.stringify(info)}`);
    }

    _mightSendBigFrames() {
      if (this._is_raw_text_mode) {
        return;
      }
      const y_diff =
        this.dimensions.frame.y_last_big_frame - this.dimensions.frame.y_scroll;
      const max_y_scroll_without_new_big_frame =
        (this.dimensions._big_sub_frame_factor - 1) *
        this.dimensions.tty.height;
      if (Math.abs(y_diff) > max_y_scroll_without_new_big_frame) {
        this.log(
          `Parsing big frames: ` +
            `previous-y: ${this.dimensions.frame.y_last_big_frame}, ` +
            `y-scroll: ${this.dimensions.frame.y_scroll}, ` +
            `diff: ${y_diff}, ` +
            `max-scroll: ${max_y_scroll_without_new_big_frame} `
        );
        this.sendAllBigFrames();
      }
    }
  };
