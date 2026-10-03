#!/usr/bin/env ruby
# frozen_string_literal: true

# Checks the measurable ASD-STE100 rules in docs/STYLE.md.
# ponytail: regex sentence split; abbreviations like "v0.1" are masked, others may miscount.

require "yaml"

MAX_WORDS = 25
MAX_SENTENCES = 6
BANNED = [
  "utilize", "utilizes", "utilized", "leverage", "leverages", "ensure", "ensures",
  "assist", "assists", "approximately", "in order to", "via", "initiate", "initiates",
  "terminate", "terminates", "prior to", "subsequent", "subsequently", "sufficient",
  "additional", "e.g.", "i.e.", "etc."
].freeze
SKIP = ["CODE_OF_CONDUCT.md"].freeze

def words(text)
  text.split(/\s+/).count { |w| w.match?(/[[:alnum:]]/) }
end

# Remove what is not prose: code spans, link targets, emphasis, HTML.
def prose(text)
  text.gsub(/`[^`]*`/, "CODE")
      .gsub(/!?\[([^\]]*)\]\([^)]*\)/, '\1')
      .gsub(/<[^>]+>/, "")
      .gsub(/\*\*|__/, "")
      .gsub(/\b(?:v?\d+(?:\.\d+)+|[A-Za-z]\.[A-Za-z]\.)/) { |m| m.delete(".") }
end

def sentences(text)
  text.split(/(?<=[.!?:])\s+(?=[A-Z"(*`\d])/).map(&:strip).reject(&:empty?)
end

def check_text(errors, where, text)
  clean = prose(text)
  errors << "#{where}: semicolon" if clean.include?(";")
  errors << "#{where}: em dash" if clean.include?("—")
  BANNED.each do |w|
    errors << "#{where}: use the approved word for \"#{w}\"" if clean.match?(/(?<![\w.])#{Regexp.escape(w)}(?![\w])/i)
  end
  ss = sentences(clean)
  ss.each do |s|
    n = words(s)
    errors << "#{where}: sentence has #{n} words (max #{MAX_WORDS}): #{s[0, 60]}..." if n > MAX_WORDS
  end
  ss.size
end

def check_markdown(errors, path)
  in_code = false
  off = false # <!-- docs-lint: off --> ... <!-- docs-lint: on --> skips bad examples
  para = []
  para_start = 0
  flush = lambda do
    unless para.empty?
      n = check_text(errors, "#{path}:#{para_start}", para.join(" "))
      errors << "#{path}:#{para_start}: paragraph has #{n} sentences (max #{MAX_SENTENCES})" if n > MAX_SENTENCES
    end
    para = []
  end

  File.readlines(path, encoding: "UTF-8").each_with_index do |line, i|
    lineno = i + 1
    if line.lstrip.start_with?("```")
      flush.call
      in_code = !in_code
      next
    end
    next if in_code

    s = line.strip
    if s.start_with?("<!-- docs-lint:")
      flush.call
      off = s.include?("off")
      next
    end
    next if off

    if s.empty? || s.start_with?("#", "|", "[!", "---")
      flush.call
      next
    end
    s = s.sub(/\A>\s?/, "")
    if s.match?(/\A([-*+]|\d+\.)\s/) # each list item is its own block
      flush.call
      para_start = lineno
      para << s.sub(/\A([-*+]|\d+\.)\s+/, "")
      next
    end
    para_start = lineno if para.empty?
    para << s
  end
  flush.call
end

def yaml_texts(node, path = [], &blk)
  case node
  when Hash then node.each { |k, v| %w[description summary].include?(k) && v.is_a?(String) ? blk.call(path + [k], v) : yaml_texts(v, path + [k], &blk) }
  when Array then node.each_with_index { |v, i| yaml_texts(v, path + [i], &blk) }
  end
end

errors = []
files = ARGV.empty? ? `git ls-files '*.md'`.split("\n") : ARGV
files.reject { |f| SKIP.include?(File.basename(f)) }.each do |f|
  if f.end_with?(".yaml", ".yml")
    yaml_texts(YAML.load_file(f)) do |p, text|
      text.split(/\n\s*\n/).each { |block| check_text(errors, "#{f}:#{p.join(".")}", block.tr("\n", " ")) }
    end
  else
    check_markdown(errors, f)
  end
end

if errors.empty?
  puts "docs-lint: ok"
else
  puts errors
  puts "docs-lint: #{errors.size} problem(s). See docs/STYLE.md."
  exit 1
end
