import { LivechatButton } from "@im_livechat/embed/common/livechat_button";
import { prettifyMessageContent } from "@mail/utils/common/format";
import { useRef } from "@odoo/owl";
import { patch } from "@web/core/utils/patch";

const WHATSAPP_PHONE_NUMBER = "524626217987";
const WHATSAPP_DEFAULT_MESSAGE = "Hola, quiero hablar con un asesor.";

LivechatButton.template = "website_vegusa.LivechatButton";

patch(LivechatButton.prototype, {
    setup() {
        super.setup(...arguments);
        this.state.text = "";
        this.inputRef = useRef("chatInput");
    },
    /** Existing livechat conversation, if the visitor already has one. */
    get activeThread() {
        return this.store.activeLivechats[0];
    },
    get hasActiveLivechat() {
        return Boolean(this.activeThread);
    },
    get canSubmit() {
        return Boolean(this.state.text.trim());
    },
    get isShown() {
        // Unlike the original bubble, the sticky bar stays visible even
        // once a conversation exists: the input turns into a "reopen"
        // button instead of disappearing.
        return this.store.livechat_available && this.livechatService.options.channel_id;
    },
    onKeydown(ev) {
        if (ev.key === "Enter") {
            ev.preventDefault();
            this.onSubmit();
        }
    },
    /** Starts a new conversation with the text typed in the sticky input. */
    async onSubmit() {
        const text = this.state.text.trim();
        if (!text) {
            return;
        }
        this.state.animateNotification = false;
        const thread = await this.livechatService.open();
        if (thread) {
            const body = await prettifyMessageContent(text);
            await thread.post(body);
        }
        this.state.text = "";
    },
    /** Reopens the existing conversation once one is active. */
    onReopen() {
        this.activeThread?.openChatWindow({ focus: true });
    },
    get whatsappUrl() {
        const text = encodeURIComponent(WHATSAPP_DEFAULT_MESSAGE);
        return `https://wa.me/${WHATSAPP_PHONE_NUMBER}?text=${text}`;
    },
    onWhatsappClick() {
        window.open(this.whatsappUrl, "_blank", "noopener,noreferrer");
    },
});
