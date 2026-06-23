import logging
import os

from dotenv import load_dotenv
from telegram import Update
from telegram.ext import (
    Application,
    CommandHandler,
    ConversationHandler,
    ContextTypes,
    MessageHandler,
    filters,
)

from naukri import (
    NaukriBot,
    NaukriError,
    NaukriSessionExpiredError,
    NaukriSelectorError,
    NaukriTimeoutError,
)

load_dotenv()

logging.basicConfig(
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    level=logging.INFO,
)
logger = logging.getLogger(__name__)

TOKEN = os.environ["TELEGRAM_BOT_TOKEN"]
ALLOWED_USER_ID = int(os.environ["ALLOWED_USER_ID"])

auth_filter = filters.User(user_id=ALLOWED_USER_ID)

# Conversation states
AWAIT_BIO, AWAIT_SKILL, AWAIT_RM_SKILL, AWAIT_HEADLINE = range(4)


# --- Error formatting ---

def _error_msg(e: NaukriError) -> str:
    if isinstance(e, NaukriSessionExpiredError):
        return "Session expired. Use /refresh_session to re-authenticate."
    if isinstance(e, NaukriSelectorError):
        return f"Naukri UI may have changed — check server logs.\nDetails: {e}"
    if isinstance(e, NaukriTimeoutError):
        return "Naukri.com didn't respond in time. Try again."
    return f"Something went wrong: {e}"


# --- /start ---

async def cmd_start(update: Update, _: ContextTypes.DEFAULT_TYPE) -> None:
    await update.message.reply_text(
        "Naukri Bot ready.\n\n"
        "/update_bio — Update profile summary\n"
        "/update_headline — Update profile headline\n"
        "/add_skill — Add a skill\n"
        "/remove_skill — Remove a skill\n"
        "/status — Check session health\n"
        "/refresh_session — Instructions to renew session\n"
        "/cancel — Cancel current operation"
    )


# --- /status ---

async def cmd_status(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    naukri: NaukriBot = context.bot_data["naukri"]
    msg = await update.message.reply_text("Checking session...")
    result = await naukri.check_status()
    if result["logged_in"]:
        await msg.edit_text(f"Session active. Profile: {result['profile_name']}")
    else:
        await msg.edit_text(
            f"Session inactive: {result['reason']}\n\nUse /refresh_session."
        )


# --- /refresh_session ---

async def cmd_refresh_session(update: Update, _: ContextTypes.DEFAULT_TYPE) -> None:
    await update.message.reply_text(
        "To refresh your Naukri session:\n\n"
        "Option A — Credentials (recommended):\n"
        "  Ensure NAUKRI_EMAIL and NAUKRI_PASSWORD are set in .env.\n"
        "  The bot re-logs in automatically when the session expires.\n\n"
        "Option B — Manual cookie export:\n"
        "  1. On your local machine run:\n"
        "     python3 save_session.py\n"
        "  2. Log in to Naukri in the browser window that opens.\n"
        "  3. SCP the saved file to the VM:\n"
        "     scp auth_state.json user@vm-ip:~/naukri-bot/\n"
        "  4. Restart the bot: pm2 restart naukri-bot"
    )


# --- /update_bio ---

async def cmd_update_bio(update: Update, _: ContextTypes.DEFAULT_TYPE) -> int:
    await update.message.reply_text(
        "Send me the new profile summary text (or /cancel to abort):"
    )
    return AWAIT_BIO


async def receive_bio(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    text = update.message.text.strip()
    if not text:
        await update.message.reply_text("Text cannot be empty. Try again or /cancel.")
        return AWAIT_BIO

    naukri: NaukriBot = context.bot_data["naukri"]
    msg = await update.message.reply_text("Updating bio...")
    try:
        await naukri.update_bio(text)
        await msg.edit_text("Bio updated successfully.")
    except NaukriError as e:
        await msg.edit_text(_error_msg(e))
    return ConversationHandler.END


# --- /update_headline ---

async def cmd_update_headline(update: Update, _: ContextTypes.DEFAULT_TYPE) -> int:
    await update.message.reply_text(
        "Send me the new headline text (or /cancel to abort):"
    )
    return AWAIT_HEADLINE


async def receive_headline(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    text = update.message.text.strip()
    if not text:
        await update.message.reply_text("Text cannot be empty. Try again or /cancel.")
        return AWAIT_HEADLINE

    naukri: NaukriBot = context.bot_data["naukri"]
    msg = await update.message.reply_text("Updating headline...")
    try:
        await naukri.update_headline(text)
        await msg.edit_text("Headline updated successfully.")
    except NaukriError as e:
        await msg.edit_text(_error_msg(e))
    return ConversationHandler.END


# --- /add_skill ---

async def cmd_add_skill(update: Update, _: ContextTypes.DEFAULT_TYPE) -> int:
    await update.message.reply_text(
        "Send me the skill name to add (or /cancel to abort):"
    )
    return AWAIT_SKILL


async def receive_add_skill(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    skill = update.message.text.strip()
    if not skill:
        await update.message.reply_text("Skill name cannot be empty. Try again or /cancel.")
        return AWAIT_SKILL

    naukri: NaukriBot = context.bot_data["naukri"]
    msg = await update.message.reply_text(f"Adding skill '{skill}'...")
    try:
        added = await naukri.add_skill(skill)
        if added:
            await msg.edit_text(f"Added '{skill}' to your Naukri skills.")
        else:
            await msg.edit_text(f"'{skill}' is already in your skills list.")
    except NaukriError as e:
        await msg.edit_text(_error_msg(e))
    return ConversationHandler.END


# --- /remove_skill ---

async def cmd_remove_skill(update: Update, _: ContextTypes.DEFAULT_TYPE) -> int:
    await update.message.reply_text(
        "Send me the skill name to remove (or /cancel to abort):"
    )
    return AWAIT_RM_SKILL


async def receive_remove_skill(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    skill = update.message.text.strip()
    if not skill:
        await update.message.reply_text("Skill name cannot be empty. Try again or /cancel.")
        return AWAIT_RM_SKILL

    naukri: NaukriBot = context.bot_data["naukri"]
    msg = await update.message.reply_text(f"Removing skill '{skill}'...")
    try:
        removed = await naukri.remove_skill(skill)
        if removed:
            await msg.edit_text(f"Removed '{skill}' from your Naukri skills.")
        else:
            await msg.edit_text(f"'{skill}' was not found in your skills list.")
    except NaukriError as e:
        await msg.edit_text(_error_msg(e))
    return ConversationHandler.END


# --- /cancel (shared fallback) ---

async def cancel_conv(update: Update, _: ContextTypes.DEFAULT_TYPE) -> int:
    await update.message.reply_text("Cancelled.")
    return ConversationHandler.END


# --- Application lifecycle ---

async def post_init(application: Application) -> None:
    naukri = NaukriBot()
    await naukri.start()
    application.bot_data["naukri"] = naukri
    logger.info("NaukriBot started and stored in bot_data")


async def post_shutdown(application: Application) -> None:
    naukri: NaukriBot = application.bot_data.get("naukri")
    if naukri:
        await naukri.stop()
        logger.info("NaukriBot shut down")


# --- Wiring ---

def build_conv(entry_cmd, entry_handler, state_id, receive_handler) -> ConversationHandler:
    fallback_filter = auth_filter & filters.COMMAND
    return ConversationHandler(
        entry_points=[CommandHandler(entry_cmd, entry_handler, filters=auth_filter)],
        states={state_id: [MessageHandler(filters.TEXT & ~filters.COMMAND & auth_filter, receive_handler)]},
        fallbacks=[
            CommandHandler("cancel", cancel_conv),
            MessageHandler(fallback_filter, cancel_conv),
        ],
        conversation_timeout=120,
    )


def main() -> None:
    app = (
        Application.builder()
        .token(TOKEN)
        .post_init(post_init)
        .post_shutdown(post_shutdown)
        .build()
    )

    app.add_handler(CommandHandler("start", cmd_start, filters=auth_filter))
    app.add_handler(CommandHandler("status", cmd_status, filters=auth_filter))
    app.add_handler(CommandHandler("refresh_session", cmd_refresh_session, filters=auth_filter))

    app.add_handler(build_conv("update_bio", cmd_update_bio, AWAIT_BIO, receive_bio))
    app.add_handler(build_conv("update_headline", cmd_update_headline, AWAIT_HEADLINE, receive_headline))
    app.add_handler(build_conv("add_skill", cmd_add_skill, AWAIT_SKILL, receive_add_skill))
    app.add_handler(build_conv("remove_skill", cmd_remove_skill, AWAIT_RM_SKILL, receive_remove_skill))

    logger.info("Bot polling started")
    app.run_polling(allowed_updates=Update.ALL_TYPES)


if __name__ == "__main__":
    main()
