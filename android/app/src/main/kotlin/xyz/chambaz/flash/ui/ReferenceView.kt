package xyz.chambaz.flash.ui

import android.graphics.Color
import android.graphics.Typeface
import android.text.style.ForegroundColorSpan
import android.text.style.StyleSpan
import android.util.TypedValue
import android.widget.TextView
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.viewinterop.AndroidView
import io.noties.markwon.Markwon
import io.noties.markwon.MarkwonConfiguration
import io.noties.markwon.RenderProps
import io.noties.markwon.ext.latex.JLatexMathPlugin
import io.noties.markwon.html.HtmlPlugin
import io.noties.markwon.html.HtmlTag
import io.noties.markwon.html.tag.SimpleTagHandler
import io.noties.markwon.inlineparser.MarkwonInlineParserPlugin
import java.text.Normalizer

private val TOKEN_RE = run {
    val d = "\\$"
    Regex("$d$d[\\s\\S]+?$d$d|$d[^$d\\n]+$d|\\*\\*[^*]+\\*\\*")
}

private object FontColorTagHandler : SimpleTagHandler() {
    override fun supportedTags() = listOf("font")
    override fun getSpans(
        configuration: MarkwonConfiguration,
        renderProps: RenderProps,
        tag: HtmlTag,
    ): Any? {
        val color = tag.attributes()["color"] ?: return null
        return try {
            arrayOf(ForegroundColorSpan(Color.parseColor(color)), StyleSpan(Typeface.BOLD))
        } catch (_: Exception) { null }
    }
}

@Composable
fun ReferenceView(
    reference: String,
    userAnswer: String,
    threshold: Double,
    modifier: Modifier = Modifier,
) {
    val context = LocalContext.current
    val errorArgb = MaterialTheme.colorScheme.error.toArgb()
    val onBgArgb = MaterialTheme.colorScheme.onBackground.toArgb()
    val textSizePx = TypedValue.applyDimension(
        TypedValue.COMPLEX_UNIT_SP, 14f, context.resources.displayMetrics
    )

    val markwon = remember(context) {
        Markwon.builder(context)
            .usePlugin(HtmlPlugin.create { plugin -> plugin.addHandler(FontColorTagHandler) })
            .usePlugin(MarkwonInlineParserPlugin.create())
            .usePlugin(
                JLatexMathPlugin.create(
                    JLatexMathPlugin.builder(textSizePx)
                        .inlinesEnabled(true)
                        .build()
                )
            )
            .build()
    }

    val md = remember(reference, userAnswer, threshold, errorArgb) {
        buildMarkdown(
            reference,
            normalizeText(userAnswer),
            threshold,
            "#%06X".format(errorArgb and 0xFFFFFF),
        )
    }

    AndroidView(
        factory = { ctx ->
            TextView(ctx).apply {
                setBackgroundColor(android.graphics.Color.TRANSPARENT)
                textSize = 14f
                setLineSpacing(0f, 1.6f)
            }
        },
        update = { tv ->
            tv.setTextColor(onBgArgb)
            markwon.setMarkdown(tv, md)
        },
        modifier = modifier,
    )
}

private fun buildMarkdown(
    reference: String,
    normAnswer: String,
    threshold: Double,
    errorHex: String,
): String {
    val sb = StringBuilder()
    var last = 0
    for (match in TOKEN_RE.findAll(reference)) {
        sb.append(reference.substring(last, match.range.first))
        val token = match.value
        when {
            token.startsWith("$") -> sb.append(token)
            token.startsWith("**") -> {
                val kw = token.substring(2, token.length - 2)
                val score = partialRatio(normalizeText(kw), normAnswer)
                if (score >= threshold) sb.append("<b>$kw</b>")
                else sb.append("<font color=\"$errorHex\">$kw</font>")
            }
        }
        last = match.range.last + 1
    }
    sb.append(reference.substring(last))
    return sb.toString()
}

private fun normalizeText(s: String): String =
    Normalizer.normalize(s.lowercase().trim(), Normalizer.Form.NFD)
        .replace(Regex("[\\u0300-\\u036f]"), "")
        .replace(Regex("[^\\w\\s]"), " ")
        .replace(Regex("\\s+"), " ")
        .trim()

private fun partialRatio(short: String, long: String): Double {
    val rs = short.toList()
    val rl = long.toList()
    if (rs.isEmpty()) return 1.0
    if (rs.size >= rl.size) return levenshteinSim(rs, rl)
    var best = 0.0
    for (i in 0..rl.size - rs.size) {
        val s = levenshteinSim(rs, rl.subList(i, i + rs.size))
        if (s > best) best = s
    }
    return best
}

private fun levenshteinSim(a: List<Char>, b: List<Char>): Double {
    if (a.isEmpty() && b.isEmpty()) return 1.0
    var prev = IntArray(b.size + 1) { it }
    var curr = IntArray(b.size + 1)
    for (i in 1..a.size) {
        curr[0] = i
        for (j in 1..b.size) {
            curr[j] = if (a[i - 1] == b[j - 1]) prev[j - 1]
                      else 1 + minOf(prev[j - 1], prev[j], curr[j - 1])
        }
        val tmp = prev; prev = curr; curr = tmp
    }
    return 1.0 - prev[b.size].toDouble() / maxOf(a.size, b.size)
}
