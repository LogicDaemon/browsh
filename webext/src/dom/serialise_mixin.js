import utils from "utils";

export default (MixinBase) =>
  class extends MixinBase {
    __serialiseFrame() {
      let cell, index;
      const top = this.dimensions.frame.sub.top / 2;
      const left = this.dimensions.frame.sub.left;
      const bottom = top + this.dimensions.frame.sub.height / 2;
      const right = left + this.dimensions.frame.sub.width;
      this._setupFrameMeta();
      this._serialiseInputBoxes();
      for (let y = top; y < bottom; y++) {
        for (let x = left; x < right; x++) {
          index = y * this.dimensions.frame.width + x;
          cell = this.tty_grid.cells[index];
          if (cell === undefined) {
            this.frame.colours.push(0);
            this.frame.colours.push(0);
            this.frame.colours.push(0);
            this.frame.text.push("");
          } else {
            cell.fg_colour.map((c) => this.frame.colours.push(c));
            this.frame.text.push(cell.rune);
          }
        }
      }
    }

    _serialiseRawText() {
      let raw_text = "";
      this._previous_cell_href = "";
      this._is_inside_anchor = false;
      const top = this.dimensions.frame.sub.top / 2;
      const left = this.dimensions.frame.sub.left;
      const bottom = top + this.dimensions.frame.sub.height / 2;
      const right = left + this.dimensions.frame.sub.width;
      for (let y = top; y < bottom; y++) {
        for (let x = left; x < right; x++) {
          raw_text += this._addCell(x, y, right);
        }
        raw_text += "\n";
      }
      return this._wrap(raw_text);
    }

    _wrap(raw_text) {
      let head;
      head =
        this._raw_mode_type === "raw_text_html"
          ? this._getHTMLHead()
          : this._getUserHeader();
      return head + raw_text + this._getFooter();
    }

    // Whether a use has shown support. This controls certain Browsh branding and
    // nags to donate.
    userHasShownSupport() {
      return (
        this.config.browsh_supporter === "I have shown my support for Browsh"
      );
    }

    _byBrowsh() {
      let by;
      if (this.userHasShownSupport()) {
        return "";
      }
      by =
        this._raw_mode_type === "raw_text_html"
          ? 'by <a href="https://www.brow.sh">Browsh</a> v'
          : "by Browsh v";
      return by + this.config.browsh_version + " ";
    }

    _getUserFooter() {
      const footer = this.config["http-server"]
        ? this.config["http-server"].footer
        : "";
      return footer ? "\n" + footer : "";
    }

    _getUserHeader() {
      const header = this.config["http-server"]
        ? this.config["http-server"].header
        : "";
      return header ? header + "\n" : "";
    }

    _getMetaData() {
      let metadata = "";
      this._markParsingDuration();
      const date_time = this._getCurrentDataTime();
      const elapsed = `${this._parsing_duration}ms`;
      metadata +=
        "\n\n" + `Built ` + this._byBrowsh() + `on ${date_time} in ${elapsed}.`;
      if (this.dimensions.is_page_truncated) {
        metadata +=
          "\nBrowsh parser: the page was too large, some text may have been truncated.";
      }
      return metadata;
    }

    _getDonateCall() {
      let donating;
      if (this.userHasShownSupport()) {
        return "";
      }
      donating =
        this._raw_mode_type === "raw_text_html"
          ? '<a href="https://www.brow.sh/donate">donating</a>'
          : "https://brow.sh/donate";
      return (
        "\nPlease consider " +
        donating +
        " to help all those with slow and/or expensive internet."
      );
    }

    _getFooter() {
      let start, end;
      if (this._raw_mode_type === "raw_text_html") {
        start = '<span class="browsh-footer">';
        end = "</span></pre></body></html>";
      } else {
        start = "";
        end = "";
      }
      return (
        start +
        this._getStatusFooter() +
        this._getUserFooter() +
        end
      );
    }

    _getStatusFooter() {
      const footerLines = [];
      if (this._raw_mode_type !== "raw_text_mcp") {
        footerLines.push(
          `The above is a text render of ${document.location.href}.`
        );
      } else {
        footerLines.push(`URL: ${document.location.href}`);
      }
      footerLines.push(this._getMouseFooterLine());
      const focusedElementFooter = this._getFocusedTextElementFooter();
      if (focusedElementFooter) {
        footerLines.push(focusedElementFooter.coordinates);
        footerLines.push(`[${focusedElementFooter.text}]`);
      }
      const selectionFooter = this._getSelectionFooterLine();
      if (selectionFooter) {
        footerLines.push(selectionFooter);
      }
      if (this.dimensions.is_page_truncated) {
        footerLines.push(
          "Browsh parser: the page was too large, some text may have been truncated."
        );
      }
      footerLines.push(
        `${this._getSecurityStatusFooterLine()}, fetched ${this._getCurrentDataTime()}.`
      );
      return (
        "\n\n" +
        footerLines
          .map((line) => this._renderFooterLine(line))
          .join("\n")
      );
    }

    _renderFooterLine(line) {
      if (this._raw_mode_type !== "raw_text_html") {
        return line;
      }
      return this._escapeHTML(line);
    }

    _escapeHTML(text) {
      return String(text)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;");
    }

    _getMouseFooterLine() {
      const mouseState = this.runtime_state
        ? this.runtime_state.last_mouse_position
        : null;
      if (!mouseState) {
        return "Mouse cursor position is unavailable.";
      }
      const mouseCoordinates = this._rawTextCoordinatesFromAbsolutePosition(
        mouseState.x,
        mouseState.y
      );
      if (!mouseCoordinates) {
        return "Mouse cursor is outside the rendered page.";
      }
      const hoveredText = this._textAtRawTextCoordinate(
        mouseCoordinates.x,
        mouseCoordinates.y
      );
      if (!hoveredText) {
        return "Mouse cursor is not over visible page text.";
      }
      return (
        `Mouse cursor is at x=${mouseCoordinates.x}, y=${mouseCoordinates.y} ` +
        `over \"${this._compactText(hoveredText, 80)}\".`
      );
    }

    _rawTextCoordinatesFromAbsolutePosition(domX, domY) {
      if (typeof domX !== "number" || typeof domY !== "number") {
        return null;
      }
      const x = utils.snap(domX * this.dimensions.scale_factor.width);
      const y = utils.snap(
        (domY * this.dimensions.scale_factor.height) / 2
      );
      if (
        x < 0 ||
        y < 0 ||
        x >= this.dimensions.frame.width ||
        y >= this.dimensions.frame.height / 2
      ) {
        return null;
      }
      return { x, y };
    }

    _textAtRawTextCoordinate(x, y) {
      const index = y * this.dimensions.frame.width + x;
      const cell = this.tty_grid.cells[index];
      if (!cell || !cell.rune || !cell.rune.trim()) {
        return "";
      }
      let start = x;
      let end = x;
      while (start > 0) {
        const previousCell = this.tty_grid.cells[y * this.dimensions.frame.width + start - 1];
        if (!previousCell || !previousCell.rune || !previousCell.rune.trim()) {
          break;
        }
        start--;
      }
      while (end + 1 < this.dimensions.frame.width) {
        const nextCell = this.tty_grid.cells[y * this.dimensions.frame.width + end + 1];
        if (!nextCell || !nextCell.rune || !nextCell.rune.trim()) {
          break;
        }
        end++;
      }
      let text = "";
      for (let i = start; i <= end; i++) {
        const lineCell = this.tty_grid.cells[y * this.dimensions.frame.width + i];
        text += lineCell && lineCell.rune ? lineCell.rune : " ";
      }
      return text.trim();
    }

    _getFocusedTextElementFooter() {
      const element = document.activeElement;
      if (!this._isFocusedTextElement(element)) {
        return null;
      }
      const cursorText = this._focusedTextWithCursor(element);
      if (!cursorText) {
        return null;
      }
      const coordinates = this._rawTextCoordinatesFromElement(element);
      if (!coordinates) {
        return null;
      }
      return {
        coordinates: `Focused text element is at x=${coordinates.x}, y=${coordinates.y}.`,
        text: cursorText,
      };
    }

    _isFocusedTextElement(element) {
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

    _focusedTextWithCursor(element) {
      if (element.tagName === "INPUT" || element.tagName === "TEXTAREA") {
        if (
          typeof element.selectionStart !== "number" ||
          typeof element.selectionEnd !== "number" ||
          element.selectionStart !== element.selectionEnd
        ) {
          return "";
        }
        return this._compactTextAroundCursor(
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
        return this._compactTextAroundCursor(
          element.textContent || "",
          range.toString().length
        );
      }
      return "";
    }

    _rawTextCoordinatesFromElement(element) {
      const domRect = this._convertDOMRectToAbsoluteCoords(
        element.getBoundingClientRect()
      );
      let top = domRect.top;
      let left = domRect.left;
      if (window.getComputedStyle) {
        const styles = window.getComputedStyle(element);
        top += parseInt(styles["padding-top"].replace("px", "")) || 0;
        left += parseInt(styles["padding-left"].replace("px", "")) || 0;
      }
      return this._rawTextCoordinatesFromAbsolutePosition(left, top);
    }

    _getSelectionFooterLine() {
      const text = this._selectedText();
      if (!text) {
        return "";
      }
      return `Selection: [${this._compactText(text, 120)}]`;
    }

    _selectedText() {
      const element = document.activeElement;
      if (
        element &&
        (element.tagName === "INPUT" || element.tagName === "TEXTAREA") &&
        typeof element.selectionStart === "number" &&
        typeof element.selectionEnd === "number" &&
        element.selectionStart !== element.selectionEnd
      ) {
        return element.value.slice(element.selectionStart, element.selectionEnd);
      }
      const selection = window.getSelection();
      if (!selection || selection.isCollapsed) {
        return "";
      }
      return selection.toString();
    }

    _compactTextAroundCursor(text, cursorIndex) {
      const characters = Array.from(text || "");
      const start = Math.max(0, cursorIndex - 40);
      const end = Math.min(characters.length, cursorIndex + 40);
      let snippet = characters.slice(start, cursorIndex).join("");
      snippet += "|";
      snippet += characters.slice(cursorIndex, end).join("");
      snippet = this._normaliseFooterWhitespace(snippet);
      if (start > 0) {
        snippet = "... " + snippet;
      }
      if (end < characters.length) {
        snippet += " ...";
      }
      return snippet;
    }

    _compactText(text, maxLength) {
      const compactText = this._normaliseFooterWhitespace(text);
      if (compactText.length <= maxLength) {
        return compactText;
      }
      return compactText.slice(0, maxLength - 4) + " ...";
    }

    _normaliseFooterWhitespace(text) {
      return String(text || "").replace(/\s+/g, " ").trim();
    }

    _getSecurityStatusFooterLine() {
      if (
        this.runtime_state &&
        this.runtime_state.security_status_text &&
        this.runtime_state.security_status_text.length > 0
      ) {
        return this.runtime_state.security_status_text;
      }
      if (document.location.protocol === "http:") {
        return "Connection is not secure";
      }
      return "Connection security status is unavailable";
    }

    _getHTMLHead() {
      const img_src = this.graphics_builder._getScaledDataURI();
      const width = this.dimensions.dom.sub.width;
      const height = this.dimensions.dom.sub.height;
      return `<html>
     <head>
       ${this._getFavicon()}
       <title>${document.title}</title>
       <style>
        html * {
         font-family: 'Courier New', monospace;
        }
        body {
          font-size: 15px;
        }
        pre {
          background-image: url(${img_src});
          background-repeat: no-repeat;
          background-size: ${width}px ${height}px;
          /* Pixelate the background image */
          image-rendering: -moz-crisp-edges;          /* Firefox                        */
          image-rendering: -o-crisp-edges;            /* Opera                          */
          image-rendering: -webkit-optimize-contrast; /* Chrome (and eventually Safari) */
          image-rendering: pixelated;                 /* Chrome                         */
          -ms-interpolation-mode: nearest-neighbor;   /* IE8+                           */
          width: ${width}px;
          height: ${height}px;
          /* These styles need to exactly follow Browsh's rendering styles */
          font-size: 15px !important;
          line-height: 20px !important;
          letter-spacing: 0px !important;
          font-style: normal !important;
          font-weight: normal !important;
        }
        .browsh-footer {
          opacity: 0.7;
        }
       </style>
     </head>
     <body>
     ${this._getUserHeader()}
     <pre>`;
    }

    _getFavicon() {
      let el = document.querySelector("link[rel*='icon']");
      if (el) {
        return `<link rel="shortcut icon" type = "image/x-icon" href="${el.href}">`;
      } else {
        return "";
      }
    }

    _markParsingDuration() {
      this._parsing_duration = performance.now() - this._parse_start_time;
    }

    _getCurrentDataTime() {
      let current_date = new Date();
      const offset = -(new Date().getTimezoneOffset() / 60);
      const sign = offset > 0 ? "+" : "-";
      let date_time =
        current_date.getDate() +
        "/" +
        (current_date.getMonth() + 1) +
        "/" +
        current_date.getFullYear() +
        "@" +
        current_date.getHours() +
        ":" +
        current_date.getMinutes() +
        ":" +
        current_date.getSeconds() +
        " " +
        "UTC" +
        sign +
        offset +
        " (" +
        Intl.DateTimeFormat().resolvedOptions().timeZone +
        ")";
      return date_time;
    }

    // TODO: Ultimately we're going to need to know exactly which parts of the input
    //       box are obscured. This is partly possible using the element's computed
    //       styles, however this isn't comprehensive - think partially obscuring.
    //       So the best solution is to use the same trick as we do for normal text,
    //       except that we can't fill the input box with text, however we can
    //       temporarily change the background to a contrasting colour.
    _getAllInputBoxes() {
      let dom_rect, styles, font_rgb;
      let parsed_input_boxes = {};
      let raw_input_boxes = document.querySelectorAll(
        "input, " + "textarea, " + '[role="textbox"]'
      );
      raw_input_boxes.forEach((i) => {
        let type;
        this._ensureBrowshID(i);
        dom_rect = this._convertDOMRectToAbsoluteCoords(
          i.getBoundingClientRect()
        );
        const width = utils.snap(
          dom_rect.width * this.dimensions.scale_factor.width
        );
        const height = utils.snap(
          dom_rect.height * this.dimensions.scale_factor.height
        );
        if (width == 0 || height == 0) {
          return;
        }
        type =
          i.getAttribute("role") == "textbox"
            ? "textbox"
            : i.getAttribute("type");
        styles = window.getComputedStyle(i);
        font_rgb = styles["color"]
          .replace(/[^\d,]/g, "")
          .split(",")
          .map((i) => parseInt(i));
        const padding_top = parseInt(styles["padding-top"].replace("px", ""));
        const padding_left = parseInt(styles["padding-left"].replace("px", ""));
        if (this._isUnwantedInboxBox(i, styles)) {
          return;
        }
        parsed_input_boxes[i.getAttribute("data-browsh-id")] = {
          id: i.getAttribute("data-browsh-id"),
          x: utils.snap(
            (dom_rect.left + padding_left) * this.dimensions.scale_factor.width
          ),
          y: utils.snap(
            (dom_rect.top + padding_top) * this.dimensions.scale_factor.height
          ),
          width: width,
          height: height,
          tag_name: i.nodeName,
          type: type,
          colour: [font_rgb[0], font_rgb[1], font_rgb[2]],
        };
      });
      return parsed_input_boxes;
    }

    _ensureBrowshID(element) {
      if (element.getAttribute("data-browsh-id") === null) {
        element.setAttribute("data-browsh-id", utils.uuidv4());
      }
    }

    _isUnwantedInboxBox(input_box, styles) {
      return (
        styles.display === "none" ||
        styles.visibility === "hidden" ||
        input_box.getAttribute("aria-hidden") == "true"
      );
    }

    _sendRawText() {
      let body;
      if (this._raw_mode_type == "raw_text_dom") {
        body =
          "<html>" +
          document.getElementsByTagName("html")[0].innerHTML +
          "</html>";
      } else {
        body = this._serialiseRawText();
      }
      let payload = {
        body: body,
        page_load_duration: this.config.page_load_duration,
        parsing_duration: this._parsing_duration,
      };
      this.sendMessage(`/raw_text,${JSON.stringify(payload)}`);
    }

    _sendFrame() {
      this._serialiseFrame();
      if (this.frame.text.length > 0) {
        this.sendMessage(`/frame_text,${JSON.stringify(this.frame)}`);
      } else {
        this.log("Not sending empty text frame");
      }
    }

    _addCell(x, y, right) {
      let text = "";
      const index = y * this.dimensions.frame.width + x;
      this._cell_for_raw_text = this.tty_grid.cells[index];
      if (this._raw_mode_type === "raw_text_html") {
        this._is_line_end = x === right - 1;
        text += this._addCellAsHTML();
      } else {
        text += this._addCellAsPlainText();
      }
      return text;
    }

    _addCellAsHTML() {
      this._HTML = "";
      if (!this._cell_for_raw_text) {
        this._addHTMLForNonExistentCell();
      } else {
        this._current_cell_href = this._cell_for_raw_text.parent_element.href;
        this._is_HREF_changed =
          this._current_cell_href !== this._previous_cell_href;
        this._handleCellOutsideAnchor();
        this._handleCellInsideAnchor();
        this._HTML += this._cell_for_raw_text.rune;
        this._previous_cell_href = this._current_cell_href;
      }
      if (this._will_be_inside_anchor !== undefined) {
        this._is_inside_anchor = this._will_be_inside_anchor;
      }
      return this._HTML;
    }

    _addHTMLForNonExistentCell() {
      if (this._is_inside_anchor) {
        this._previous_cell_href = undefined;
        this._closeAnchorTag();
      }
      this._HTML += " ";
    }

    _handleCellOutsideAnchor() {
      if (this._is_inside_anchor) {
        return;
      }
      if (this._current_cell_href || this._is_HREF_changed) {
        this._openAnchorTag();
      }
    }

    _handleCellInsideAnchor() {
      if (!this._is_inside_anchor) {
        return;
      }
      if (
        this._is_HREF_changed ||
        !this._current_cell_href ||
        this._is_line_end
      ) {
        this._closeAnchorTag();
        if (this._current_cell_href) {
          this._openAnchorTag();
        }
      }
    }

    _openAnchorTag() {
      this._will_be_inside_anchor = true;
      this._HTML += `<a href="/${this._current_cell_href}">`;
    }

    _closeAnchorTag() {
      this._will_be_inside_anchor = false;
      this._HTML += `</a>`;
    }

    _addCellAsPlainText() {
      if (this._cell_for_raw_text === undefined) {
        return " ";
      }
      return this._cell_for_raw_text.rune;
    }

    _setupFrameMeta() {
      this.frame = {
        meta: this.dimensions.getFrameMeta(),
        text: [],
        colours: [],
      };
      this.frame.meta.id = parseInt(this.channel.name);
    }

    _serialiseInputBoxes() {
      this.frame.input_boxes = this._getAllInputBoxes();
    }
  };
